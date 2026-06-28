// SPDX-License-Identifier: GPL-3.0-or-later

package installer

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ElVatoEste/Bindle/internal/manifest"
	"github.com/ElVatoEste/Bindle/internal/transport"
)

// mockHost records CL commands and serves a canned signature for DSPSRVPGM.
type mockHost struct {
	signature string
	cls       []string
	uploads   int
}

func (m *mockHost) Run(cmd string) (transport.Result, error) {
	if strings.Contains(cmd, "echo $HOME") {
		return transport.Result{Stdout: "/home/VATODEV\n"}, nil
	}
	return transport.Result{}, nil
}

func (m *mockHost) RunCL(cl string) (transport.Result, error) {
	m.cls = append(m.cls, cl)
	if strings.Contains(cl, "DSPSRVPGM") {
		return transport.Result{Stdout: "Signatures:\n " + m.signature + "\n"}, nil
	}
	return transport.Result{}, nil
}

func (m *mockHost) Upload(local, remote string) error   { m.uploads++; return nil }
func (m *mockHost) Download(remote, local string) error { return nil }

func lockWith(sig string) *manifest.Lock {
	l := manifest.NewLock()
	l.Resolved["modgreet"] = manifest.LockEntry{
		Version:   "0.1.0",
		Library:   "MODGREET",
		Srvpgm:    "HELLOSRV",
		Signature: sig,
		Artifact:  "modgreet/0.1.0/HELLOSRV.savf",
		Hash:      "sha256:x",
	}
	return l
}

func seedCache(t *testing.T) string {
	t.Helper()
	cache := t.TempDir()
	p := filepath.Join(cache, "modgreet", "0.1.0", "HELLOSRV.savf")
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte("SAVF"), 0o644); err != nil {
		t.Fatal(err)
	}
	return cache
}

func TestDeployRestoresAndVerifies(t *testing.T) {
	sig := "0000000000000000000000E3C5C5D9C7"
	h := &mockHost{signature: sig}
	cache := seedCache(t)

	got, err := Deploy(h, lockWith(sig), DeployOptions{CacheDir: cache, WireLibList: true})
	if err != nil {
		t.Fatalf("deploy: %v", err)
	}
	if len(got) != 1 || got[0].Srvpgm != "HELLOSRV" {
		t.Fatalf("deployed = %+v", got)
	}
	if h.uploads != 1 {
		t.Errorf("uploads = %d, want 1", h.uploads)
	}
	joined := strings.Join(h.cls, "\n")
	for _, want := range []string{"CRTSAVF", "CPYFRMSTMF", "RSTOBJ", "DSPSRVPGM", "ADDLIBLE"} {
		if !strings.Contains(joined, want) {
			t.Errorf("expected a %s command; got:\n%s", want, joined)
		}
	}
}

func TestDeploySignatureMismatchAborts(t *testing.T) {
	h := &mockHost{signature: "DEADBEEF00000000000000000000DEAD"}
	cache := seedCache(t)

	_, err := Deploy(h, lockWith("0000000000000000000000E3C5C5D9C7"), DeployOptions{CacheDir: cache})
	var mm *SignatureMismatchError
	if !errors.As(err, &mm) {
		t.Fatalf("expected *SignatureMismatchError, got %T: %v", err, err)
	}
	if mm.Package != "modgreet" {
		t.Errorf("mismatch package = %q", mm.Package)
	}
}

func TestDeployLibraryOverride(t *testing.T) {
	sig := "0000000000000000000000E3C5C5D9C7"
	h := &mockHost{signature: sig}
	cache := seedCache(t)

	got, err := Deploy(h, lockWith(sig), DeployOptions{CacheDir: cache, LibraryOverride: "VATODEV1"})
	if err != nil {
		t.Fatalf("deploy: %v", err)
	}
	if got[0].Library != "VATODEV1" {
		t.Errorf("library = %q, want VATODEV1 (override)", got[0].Library)
	}
}
