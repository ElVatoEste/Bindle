// SPDX-License-Identifier: GPL-3.0-or-later

// Package builder compiles a module's RPG sources into ILE objects on an IBM i
// host, extracts the service program signature, and packages a SAVF.
//
// It drives the host directly with CL (no Bob dependency): upload source over
// SFTP, CRTRPGMOD + CRTSRVPGM, read the signature via DSPSRVPGM, then
// SAVOBJ -> CPYTOSTMF -> download.
package builder

import (
	"fmt"
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

	// 1. upload + tag + compile each module
	for _, mod := range modules {
		local := path.Join(filepathToSlash(opts.SourceDir), mod+".rpgle")
		remote := path.Join(workDir, mod+".rpgle")
		logf("upload %s", mod)
		if err := h.Upload(fromSlash(local), remote); err != nil {
			return nil, fmt.Errorf("upload %s: %w", mod, err)
		}
		if _, err := h.Run(fmt.Sprintf("setccsid 1252 %s", shellQuote(remote))); err != nil {
			return nil, fmt.Errorf("setccsid %s: %w", mod, err)
		}
		_, _ = h.RunCL(fmt.Sprintf("DLTMOD MODULE(%s/%s)", lib, mod)) // ignore if absent
		logf("compile module %s", mod)
		if err := cl(h, "CRTRPGMOD",
			"CRTRPGMOD MODULE(%s/%s) SRCSTMF('%s') TGTCCSID(*JOB)", lib, mod, remote); err != nil {
			return nil, err
		}
	}

	// 2. (re)create the service program
	_, _ = h.RunCL(fmt.Sprintf("DLTSRVPGM SRVPGM(%s/%s)", lib, srv))
	modList := strings.Join(qualify(lib, modules), " ")
	logf("create service program %s", srv)
	if err := cl(h, "CRTSRVPGM",
		"CRTSRVPGM SRVPGM(%s/%s) MODULE(%s) EXPORT(*ALL)", lib, srv, modList); err != nil {
		return nil, err
	}

	// 3. extract signature
	sig, err := signatureOf(h, lib, srv)
	if err != nil {
		return nil, err
	}
	logf("signature %s", sig)

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
func CL(h Host, label, command string) error {
	r, err := h.RunCL(command)
	if err != nil {
		return fmt.Errorf("%s: %w", label, err)
	}
	if r.Failed() {
		return fmt.Errorf("%s failed:\n%s", label, tail(r.Stdout, r.Stderr))
	}
	return nil
}

// Signature reads the current signature of a service program via DSPSRVPGM.
func Signature(h Host, lib, srv string) (string, error) { return signatureOf(h, lib, srv) }

// signatureOf reads the current signature of a service program via DSPSRVPGM.
func signatureOf(h Host, lib, srv string) (string, error) {
	r, err := h.RunCL(fmt.Sprintf("DSPSRVPGM SRVPGM(%s/%s) DETAIL(*SIGNATURE)", lib, srv))
	if err != nil {
		return "", fmt.Errorf("DSPSRVPGM: %w", err)
	}
	if r.Failed() {
		return "", fmt.Errorf("DSPSRVPGM failed:\n%s", tail(r.Stdout, r.Stderr))
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
