// SPDX-License-Identifier: GPL-3.0-or-later

// Package builder compiles a module's RPG sources into ILE objects on an IBM i
// host, extracts the service program signature, and packages a SAVF.
//
// It drives the host directly with CL (no Bob dependency): upload source over
// SFTP, CRTRPGMOD + CRTSRVPGM, read the signature via DSPSRVPGM, then
// SAVOBJ -> CPYTOSTMF -> download.
package builder

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path"
	"regexp"
	"strings"

	"github.com/ElVatoEste/Bindle/internal/manifest"
	"github.com/ElVatoEste/Bindle/internal/transport"
)

// Host is the slice of the SSH transport the builder needs.
type Host interface {
	Run(cmd string) (transport.Result, error)
	RunCL(cl string) (transport.Result, error)
	Upload(localPath, remotePath string) error
	Download(remotePath, localPath string) error
}

// Options configures a remote build.
type Options struct {
	Manifest   *manifest.Manifest
	SourceDir  string // local dir holding the module's source files
	TargetLib  string // IBM i library to build into (overrides manifest.Library)
	OutputPath string // local path to write the resulting .savf
	Keep       bool   // keep remote work objects/dir for debugging
	Logf       func(string, ...any)
}

// Result is the outcome of a build.
type Result struct {
	Signature string   // current *SRVPGM signature (hex, e.g. "0000...E3C5C5D9C7")
	SavfPath  string   // local path of the packaged SAVF
	TargetLib string   // library the objects were built into
	Srvpgm    string   // service program name
	Modules   []string // module names built
}

var sigLineRe = regexp.MustCompile(`^[0-9A-Fa-f]{32}$`)

// Build runs the full remote build pipeline and downloads the SAVF.
func Build(h Host, opts Options) (*Result, error) {
	m := opts.Manifest
	if m == nil {
		return nil, fmt.Errorf("build: manifest is required")
	}
	if m.Exports == nil || m.Exports.Srvpgm == "" {
		return nil, fmt.Errorf("build: manifest must declare exports.srvpgm")
	}
	modules := modulesOf(m)
	if len(modules) == 0 {
		return nil, fmt.Errorf("build: no modules to compile (set build.objects)")
	}
	lib := opts.TargetLib
	if lib == "" {
		lib = m.Library
	}
	srv := m.Exports.Srvpgm
	logf := opts.Logf
	if logf == nil {
		logf = func(string, ...any) {}
	}

	home, err := remoteHome(h)
	if err != nil {
		return nil, err
	}
	workDir := path.Join(home, ".bindle", "build", m.Name)
	// Scratch SAVF name, deliberately distinct from the srvpgm/module object
	// names to avoid same-name collisions in the target library.
	savfName := "BNDLSAVF"

	// Best-effort: ensure the target library exists (ignored if it already does
	// or the host forbids CRTLIB — then it must pre-exist).
	_, _ = h.RunCL(fmt.Sprintf("CRTLIB LIB(%s) TEXT('Bindle %s')", lib, m.Name))

	if _, err := h.Run("mkdir -p " + shellQuote(workDir)); err != nil {
		return nil, fmt.Errorf("create remote workdir: %w", err)
	}

	// Determine export symbols + a deterministic signature BEFORE compiling, so
	// Bindle controls the signature rather than reading an auto-generated one.
	symbols, err := exportSymbols(m, opts.SourceDir, modules)
	if err != nil {
		return nil, err
	}
	major, err := majorVersion(m)
	if err != nil {
		return nil, err
	}
	sig := DeterministicSignature(m.Name, major)

	// 1. upload + tag + compile each module
	for _, mod := range modules {
		local := path.Join(filepathToSlash(opts.SourceDir), mod+".rpgle")
		remote := path.Join(workDir, mod+".rpgle")
		logf("upload %s", mod)
		if err := h.Upload(fromSlash(local), remote); err != nil {
			return nil, fmt.Errorf("upload %s: %w", mod, err)
		}
		if _, err := h.Run(fmt.Sprintf("setccsid 819 %s", shellQuote(remote))); err != nil {
			return nil, fmt.Errorf("setccsid %s: %w", mod, err)
		}
		_, _ = h.RunCL(fmt.Sprintf("DLTMOD MODULE(%s/%s)", lib, mod)) // ignore if absent
		logf("compile module %s", mod)
		if err := cl(h, "CRTRPGMOD",
			"CRTRPGMOD MODULE(%s/%s) SRCSTMF('%s') TGTCCSID(*JOB)", lib, mod, remote); err != nil {
			return nil, err
		}
	}

	// 2. generate binder source with the explicit signature, upload it, and bind.
	binderRemote := path.Join(workDir, srv+".bnd")
	if err := uploadText(h, binderSource(sig, symbols), binderRemote); err != nil {
		return nil, fmt.Errorf("upload binder source: %w", err)
	}
	_, _ = h.RunCL(fmt.Sprintf("DLTSRVPGM SRVPGM(%s/%s)", lib, srv))
	modList := strings.Join(qualify(lib, modules), " ")
	logf("create service program %s (signature %s)", srv, sig)
	if err := cl(h, "CRTSRVPGM",
		"CRTSRVPGM SRVPGM(%s/%s) MODULE(%s) EXPORT(*SRCFILE) SRCSTMF('%s')",
		lib, srv, modList, binderRemote); err != nil {
		return nil, err
	}

	// 3. sanity-check: the built signature must equal the value Bindle wrote.
	if got, err := signatureOf(h, lib, srv); err == nil {
		if !strings.HasSuffix(strings.ToUpper(got), sig) && !strings.EqualFold(got, sig) {
			logf("warning: host signature %s != declared %s", got, sig)
		}
	}

	// 4. package: SAVF in the target lib, then copy to IFS
	savfRemote := path.Join(workDir, savfName+".savf")
	_, _ = h.RunCL(fmt.Sprintf("DLTF FILE(%s/%s)", lib, savfName))
	if err := cl(h, "CRTSAVF", "CRTSAVF FILE(%s/%s)", lib, savfName); err != nil {
		return nil, err
	}
	objList := strings.Join(append([]string{srv}, modules...), " ")
	logf("save objects")
	if err := cl(h, "SAVOBJ",
		"SAVOBJ OBJ(%s) LIB(%s) DEV(*SAVF) SAVF(%s/%s) OBJTYPE(*SRVPGM *MODULE)",
		objList, lib, lib, savfName); err != nil {
		return nil, err
	}
	qsysPath := fmt.Sprintf("/QSYS.LIB/%s.LIB/%s.FILE", lib, savfName)
	if err := cl(h, "CPYTOSTMF",
		"CPYTOSTMF FROMMBR('%s') TOSTMF('%s') STMFOPT(*REPLACE) CVTDTA(*NONE)",
		qsysPath, savfRemote); err != nil {
		return nil, err
	}

	// 5. download
	logf("download %s", opts.OutputPath)
	if err := h.Download(savfRemote, opts.OutputPath); err != nil {
		return nil, fmt.Errorf("download savf: %w", err)
	}

	if !opts.Keep {
		_, _ = h.RunCL(fmt.Sprintf("DLTF FILE(%s/%s)", lib, savfName))
		_, _ = h.Run("rm -rf " + shellQuote(workDir))
	}

	return &Result{
		Signature: sig,
		SavfPath:  opts.OutputPath,
		TargetLib: lib,
		Srvpgm:    srv,
		Modules:   modules,
	}, nil
}

// cl runs a CL command and fails on either a transport error or a non-zero CL
// exit (RunCL reports CL escape messages via the exit code, not a Go error).
func cl(h Host, label, format string, a ...any) error {
	return CL(h, label, fmt.Sprintf(format, a...))
}

// CL runs a single CL command and returns an error on a transport failure or a
// non-zero CL exit. Shared with other packages that drive the host (e.g. deploy).
// On CL failure the error includes the IBM i diagnostics found in the output.
func CL(h Host, label, command string) error {
	r, err := h.RunCL(command)
	if err != nil {
		return fmt.Errorf("%s: %w", label, err)
	}
	if r.Failed() {
		return errStr(FormatDiagnostics(label, r.Stdout, r.Stderr))
	}
	return nil
}

// errStr wraps a preformatted diagnostic string as an error.
func errStr(s string) error { return &diagError{s} }

type diagError struct{ s string }

func (e *diagError) Error() string { return e.s }

// Signature reads the current signature of a service program via DSPSRVPGM.
func Signature(h Host, lib, srv string) (string, error) { return signatureOf(h, lib, srv) }

// signatureOf reads the current signature of a service program via DSPSRVPGM.
func signatureOf(h Host, lib, srv string) (string, error) {
	r, err := h.RunCL(fmt.Sprintf("DSPSRVPGM SRVPGM(%s/%s) DETAIL(*SIGNATURE)", lib, srv))
	if err != nil {
		return "", fmt.Errorf("DSPSRVPGM: %w", err)
	}
	if r.Failed() {
		return "", errStr(FormatDiagnostics("DSPSRVPGM", r.Stdout, r.Stderr))
	}
	return parseSignature(r.Stdout)
}

// parseSignature pulls the first 32-hex-digit signature from DSPSRVPGM output.
func parseSignature(out string) (string, error) {
	seenHeader := false
	for _, line := range strings.Split(out, "\n") {
		f := strings.TrimSpace(line)
		if strings.Contains(f, "Signatures:") {
			seenHeader = true
			continue
		}
		if seenHeader && sigLineRe.MatchString(f) {
			return strings.ToUpper(f), nil
		}
	}
	return "", fmt.Errorf("could not find a signature in DSPSRVPGM output")
}

func modulesOf(m *manifest.Manifest) []string {
	if m.Build != nil && len(m.Build.Objects) > 0 {
		return m.Build.Objects
	}
	return nil
}

var exportProcRe = regexp.MustCompile(`(?im)^\s*dcl-proc\s+(\w+)\b[^;]*\bexport\b`)

// exportSymbols returns the public export symbols, from manifest.exports.symbols
// if given, otherwise scanned from the module sources (dcl-proc ... export).
// Symbols are uppercased to match ILE's default exported names.
func exportSymbols(m *manifest.Manifest, sourceDir string, modules []string) ([]string, error) {
	if m.Exports != nil && len(m.Exports.Symbols) > 0 {
		out := make([]string, len(m.Exports.Symbols))
		for i, s := range m.Exports.Symbols {
			out[i] = strings.ToUpper(s)
		}
		return out, nil
	}
	var syms []string
	seen := map[string]bool{}
	for _, mod := range modules {
		data, err := os.ReadFile(fromSlash(path.Join(filepathToSlash(sourceDir), mod+".rpgle")))
		if err != nil {
			return nil, fmt.Errorf("scan exports in %s: %w", mod, err)
		}
		for _, mt := range exportProcRe.FindAllStringSubmatch(string(data), -1) {
			s := strings.ToUpper(mt[1])
			if !seen[s] {
				seen[s] = true
				syms = append(syms, s)
			}
		}
	}
	if len(syms) == 0 {
		return nil, fmt.Errorf("no exported procedures found (add exports.symbols or `dcl-proc ... export`)")
	}
	return syms, nil
}

// DeterministicSignature derives a stable 16-byte signature (32 hex chars) from
// the module name and MAJOR version. It is stable across minor/patch releases
// (binary-compatible) and changes only on a major bump (breaking).
func DeterministicSignature(name string, major uint64) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s@%d", name, major)))
	return strings.ToUpper(hex.EncodeToString(sum[:16]))
}

func majorVersion(m *manifest.Manifest) (uint64, error) {
	v, err := m.SemVer()
	if err != nil {
		return 0, fmt.Errorf("version: %w", err)
	}
	return v.Major(), nil
}

// binderSource generates ILE binder language with an explicit signature.
// New exports must be appended (keeping order) so the signature stays stable.
func binderSource(sig string, symbols []string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "STRPGMEXP PGMLVL(*CURRENT) SIGNATURE(X'%s')\n", sig)
	for _, s := range symbols {
		fmt.Fprintf(&b, "  EXPORT SYMBOL('%s')\n", s)
	}
	b.WriteString("ENDPGMEXP\n")
	return b.String()
}

// uploadText writes text to a temp local file and uploads it, then tags CCSID 819.
func uploadText(h Host, content, remote string) error {
	tmp, err := os.CreateTemp("", "bindle-*.bnd")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.WriteString(content); err != nil {
		tmp.Close()
		return err
	}
	tmp.Close()
	if err := h.Upload(tmp.Name(), remote); err != nil {
		return err
	}
	_, err = h.Run(fmt.Sprintf("setccsid 819 %s", shellQuote(remote)))
	return err
}

func qualify(lib string, names []string) []string {
	out := make([]string, len(names))
	for i, n := range names {
		out[i] = lib + "/" + n
	}
	return out
}

func remoteHome(h Host) (string, error) {
	r, err := h.Run("echo $HOME")
	if err != nil {
		return "", fmt.Errorf("determine remote home: %w", err)
	}
	home := strings.TrimSpace(r.Stdout)
	if home == "" {
		return "", fmt.Errorf("remote $HOME is empty")
	}
	return home, nil
}

func shellQuote(s string) string { return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'" }

// filepathToSlash / fromSlash bridge local OS paths and the forward-slash paths
// used for remote/IFS locations.
func filepathToSlash(p string) string { return strings.ReplaceAll(p, "\\", "/") }
func fromSlash(p string) string       { return p }

func tail(stdout, stderr string) string {
	s := strings.TrimSpace(stderr)
	if s == "" {
		s = strings.TrimSpace(stdout)
	}
	return s
}
