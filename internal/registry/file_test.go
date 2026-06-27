// SPDX-License-Identifier: GPL-3.0-or-later

package registry

import (
	"os"
	"path/filepath"
	"testing"
)

func writePkg(t *testing.T, root, name, body string) {
	t.Helper()
	dir := filepath.Join(root, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, VersionsFile), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestFileVersions(t *testing.T) {
	root := t.TempDir()
	writePkg(t, root, "modfact", `{
	  "name": "modfact",
	  "versions": [
	    {"version":"2.3.0","signature":"F23","artifact":"a.savf","hash":"sha256:x",
	     "dependencies":{"modbase":">=1.0.0 <2.0.0"}},
	    {"version":"2.2.0","yanked":true}
	  ]
	}`)

	reg := Open(root)
	got, err := reg.Versions("modfact")
	if err != nil {
		t.Fatalf("Versions: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d versions, want 1 (yanked skipped)", len(got))
	}
	v := got[0]
	if v.Version != "2.3.0" || v.Signature != "F23" || v.Hash != "sha256:x" {
		t.Errorf("unexpected version meta: %+v", v)
	}
	if v.Dependencies["modbase"] != ">=1.0.0 <2.0.0" {
		t.Errorf("deps not parsed: %+v", v.Dependencies)
	}
}

func TestFileVersionsMissingPackage(t *testing.T) {
	reg := Open(t.TempDir())
	got, err := reg.Versions("ghost")
	if err != nil {
		t.Fatalf("missing package should not error, got %v", err)
	}
	if got != nil {
		t.Errorf("missing package should yield nil, got %v", got)
	}
}
