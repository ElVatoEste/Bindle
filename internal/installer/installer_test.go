// SPDX-License-Identifier: GPL-3.0-or-later

package installer

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/ElVatoEste/Bindle/internal/manifest"
	"github.com/ElVatoEste/Bindle/internal/resolver"
)

type mockReg struct {
	versions map[string][]resolver.Available
	blobs    map[string][]byte
}

func (m *mockReg) Versions(name string) ([]resolver.Available, error) { return m.versions[name], nil }

func (m *mockReg) Fetch(artifact string) ([]byte, error) {
	b, ok := m.blobs[artifact]
	if !ok {
		return nil, fmt.Errorf("artifact not found: %s", artifact)
	}
	return b, nil
}

func sha(b []byte) string {
	sum := sha256.Sum256(b)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func writeManifest(t *testing.T, dir string) string {
	t.Helper()
	path := filepath.Join(dir, manifest.FileName)
	body := `{"schema":"bindle/v0","name":"miapp","version":"0.1.0","private":true,
	  "library":"MIAPP","dependencies":{"modfact":"^2.3.0"}}`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// goodReg returns a registry where modfact has an artifact and modbase does not.
func goodReg() *mockReg {
	factBlob := []byte("FACT-ARTIFACT")
	return &mockReg{
		versions: map[string][]resolver.Available{
			"modfact": {{Version: "2.3.0", Artifact: "modfact/2.3.0/MODFACT.savf", Hash: sha(factBlob),
				Dependencies: map[string]string{"modbase": "^1.0.0"}}},
			"modbase": {{Version: "1.4.2"}}, // no artifact
		},
		blobs: map[string][]byte{"modfact/2.3.0/MODFACT.savf": factBlob},
	}
}

func TestInstallWritesLockAndFetches(t *testing.T) {
	dir := t.TempDir()
	mPath := writeManifest(t, dir)
	cache := filepath.Join(dir, "cache")

	res, err := Install(goodReg(), Options{ManifestPath: mPath, CacheDir: cache})
	if err != nil {
		t.Fatalf("install: %v", err)
	}
	if !res.LockWritten {
		t.Error("expected lock to be written on first install")
	}
	if _, err := os.Stat(filepath.Join(dir, manifest.LockFileName)); err != nil {
		t.Errorf("lock file not created: %v", err)
	}
	if len(res.Fetched) != 1 {
		t.Fatalf("fetched = %d, want 1", len(res.Fetched))
	}
	if res.Fetched[0].Name != "modfact" {
		t.Errorf("fetched %q, want modfact", res.Fetched[0].Name)
	}
	got, err := os.ReadFile(res.Fetched[0].Path)
	if err != nil || string(got) != "FACT-ARTIFACT" {
		t.Errorf("cached artifact wrong: %q err=%v", got, err)
	}
	if len(res.Skipped) != 1 || res.Skipped[0] != "modbase" {
		t.Errorf("skipped = %v, want [modbase]", res.Skipped)
	}
}

func TestInstallReusesLock(t *testing.T) {
	dir := t.TempDir()
	mPath := writeManifest(t, dir)
	cache := filepath.Join(dir, "cache")

	if _, err := Install(goodReg(), Options{ManifestPath: mPath, CacheDir: cache}); err != nil {
		t.Fatalf("first install: %v", err)
	}
	res, err := Install(goodReg(), Options{ManifestPath: mPath, CacheDir: cache})
	if err != nil {
		t.Fatalf("second install: %v", err)
	}
	if res.LockWritten {
		t.Error("second install should reuse the existing lock, not rewrite it")
	}
	if len(res.Fetched) != 1 {
		t.Errorf("fetched = %d, want 1 (still fetches from lock)", len(res.Fetched))
	}
}

func TestInstallHashMismatch(t *testing.T) {
	dir := t.TempDir()
	mPath := writeManifest(t, dir)

	reg := goodReg()
	// corrupt the stored blob so its digest no longer matches the declared hash
	reg.blobs["modfact/2.3.0/MODFACT.savf"] = []byte("TAMPERED")

	_, err := Install(reg, Options{ManifestPath: mPath, CacheDir: filepath.Join(dir, "cache")})
	var hm *HashMismatchError
	if !errors.As(err, &hm) {
		t.Fatalf("expected *HashMismatchError, got %T: %v", err, err)
	}
	if hm.Package != "modfact" {
		t.Errorf("mismatch package = %q, want modfact", hm.Package)
	}
}
