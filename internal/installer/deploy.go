// SPDX-License-Identifier: GPL-3.0-or-later

package installer

import (
	"fmt"
	"path"
	"strings"

	"github.com/ElVatoEste/Bindle/internal/builder"
	"github.com/ElVatoEste/Bindle/internal/manifest"
)

// Host is the slice of the SSH transport the deployer needs.
type Host interface {
	builder.Host
}

// DeployOptions configures deployment of cached artifacts onto an IBM i host.
type DeployOptions struct {
	CacheDir        string // where Install wrote artifacts
	LibraryOverride string // force a single target library (e.g. on pub400)
	WireLibList     bool   // ADDLIBLE each deployed library
	Logf            func(string, ...any)
}

// Deployed records one package restored to the host.
type Deployed struct {
	Name      string
	Version   string
	Library   string
	Srvpgm    string
	Signature string
}

// SignatureMismatchError is returned when a restored service program's signature
// does not match the lock. Deployment aborts rather than wire a broken binding.
type SignatureMismatchError struct {
	Package, Want, Got string
}

func (e *SignatureMismatchError) Error() string {
	return fmt.Sprintf("%s: signature mismatch: lock has %s, host has %s", e.Package, e.Want, e.Got)
}

// Deploy restores each locked artifact onto the host, verifies its service
// program signature against the lock, and optionally wires the library list.
func Deploy(h Host, lock *manifest.Lock, opts DeployOptions) ([]Deployed, error) {
	logf := opts.Logf
	if logf == nil {
		logf = func(string, ...any) {}
	}

	home, err := remoteHome(h)
	if err != nil {
		return nil, err
	}
	workDir := path.Join(home, ".bindle", "deploy")
	if _, err := h.Run("mkdir -p '" + workDir + "'"); err != nil {
		return nil, fmt.Errorf("create remote workdir: %w", err)
	}

	var out []Deployed
	for _, name := range sortedKeys(lock.Resolved) {
		e := lock.Resolved[name]
		if e.Artifact == "" {
			continue
		}
		lib := e.Library
		if opts.LibraryOverride != "" {
			lib = opts.LibraryOverride
		}
		if lib == "" {
			return nil, fmt.Errorf("%s: no target library (publish with --library or set manifest.library)", name)
		}

		localSavf := path.Join(filepathToSlash(opts.CacheDir), name, e.Version, path.Base(e.Artifact))
		remoteSavf := path.Join(workDir, name+".savf")
		logf("upload %s@%s", name, e.Version)
		if err := h.Upload(fromSlash(localSavf), remoteSavf); err != nil {
			return nil, fmt.Errorf("upload %s: %w", name, err)
		}

		// stream file -> SAVF object -> restore
		scratch := "BNDLRST"
		_ = builder.CL(h, "DLTF", fmt.Sprintf("DLTF FILE(%s/%s)", lib, scratch))
		if err := builder.CL(h, "CRTSAVF", fmt.Sprintf("CRTSAVF FILE(%s/%s)", lib, scratch)); err != nil {
			return nil, err
		}
		if err := builder.CL(h, "CPYFRMSTMF", fmt.Sprintf(
			"CPYFRMSTMF FROMSTMF('%s') TOMBR('/QSYS.LIB/%s.LIB/%s.FILE') MBROPT(*REPLACE) CVTDTA(*NONE)",
			remoteSavf, lib, scratch)); err != nil {
			return nil, err
		}
		logf("restore into %s", lib)
		if err := builder.CL(h, "RSTOBJ", fmt.Sprintf(
			"RSTOBJ OBJ(*ALL) SAVLIB(%s) DEV(*SAVF) SAVF(%s/%s) RSTLIB(%s) MBROPT(*ALL) ALWOBJDIF(*ALL)",
			lib, lib, scratch, lib)); err != nil {
			return nil, err
		}
		_ = builder.CL(h, "DLTF", fmt.Sprintf("DLTF FILE(%s/%s)", lib, scratch))

		// signature verification against the lock
		if e.Signature != "" && e.Srvpgm != "" {
			got, err := builder.Signature(h, lib, e.Srvpgm)
			if err != nil {
				return nil, err
			}
			if !strings.EqualFold(got, e.Signature) {
				return nil, &SignatureMismatchError{Package: name, Want: e.Signature, Got: got}
			}
			logf("signature verified %s", got)
		}

		if opts.WireLibList {
			_ = builder.CL(h, "ADDLIBLE", fmt.Sprintf("ADDLIBLE LIB(%s)", lib))
		}

		out = append(out, Deployed{
			Name: name, Version: e.Version, Library: lib, Srvpgm: e.Srvpgm, Signature: e.Signature,
		})
	}
	return out, nil
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

func filepathToSlash(p string) string { return strings.ReplaceAll(p, "\\", "/") }
func fromSlash(p string) string       { return p }
