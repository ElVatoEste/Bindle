// SPDX-License-Identifier: GPL-3.0-or-later

package manifest

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLockRoundTrip(t *testing.T) {
	l := NewLock()
	l.Resolved["modfact"] = LockEntry{
		Version:      "2.3.0",
		Signature:    "A1B2C3",
		Artifact:     "ifs:///bindle/registry/modfact/2.3.0/MODFACT.savf",
		Hash:         "sha256:deadbeef",
		Dependencies: []string{"modbase", "modimp"},
	}
	l.Resolved["modbase"] = LockEntry{Version: "1.4.2", Signature: "9F8E"}

	path := filepath.Join(t.TempDir(), LockFileName)
	if err := l.Save(path); err != nil {
		t.Fatalf("save: %v", err)
	}

	got, err := LoadLock(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got.Schema != LockSchemaV0 {
		t.Errorf("schema = %q", got.Schema)
	}
	if len(got.Resolved) != 2 {
		t.Fatalf("resolved count = %d, want 2", len(got.Resolved))
	}
	if got.Resolved["modfact"].Hash != "sha256:deadbeef" {
		t.Errorf("modfact hash not preserved: %+v", got.Resolved["modfact"])
	}
}

func TestLockSaveIsDeterministic(t *testing.T) {
	build := func() *Lock {
		l := NewLock()
		l.Resolved["zeta"] = LockEntry{Version: "1.0.0"}
		l.Resolved["alpha"] = LockEntry{Version: "2.0.0"}
		l.Resolved["mid"] = LockEntry{Version: "3.0.0"}
		return l
	}

	dir := t.TempDir()
	p1 := filepath.Join(dir, "a.lock")
	p2 := filepath.Join(dir, "b.lock")
	if err := build().Save(p1); err != nil {
		t.Fatal(err)
	}
	if err := build().Save(p2); err != nil {
		t.Fatal(err)
	}

	b1, _ := os.ReadFile(p1)
	b2, _ := os.ReadFile(p2)
	if string(b1) != string(b2) {
		t.Errorf("lock output not deterministic:\n--- a ---\n%s\n--- b ---\n%s", b1, b2)
	}
}

func TestLoadLockRejectsBadSchema(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.lock")
	if err := os.WriteFile(path, []byte(`{"schema":"nope","resolved":{}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadLock(path); err == nil {
		t.Fatal("expected error for bad lock schema")
	}
}