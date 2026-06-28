// SPDX-License-Identifier: GPL-3.0-or-later

package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/ElVatoEste/Bindle/internal/manifest"
)

func setupAddProject(t *testing.T) (manifestPath, regDir string) {
	t.Helper()
	dir := t.TempDir()
	regDir = filepath.Join(dir, "registry")
	d := filepath.Join(regDir, "modfact")
	if err := os.MkdirAll(d, 0o755); err != nil {
		t.Fatal(err)
	}
	body := `{"name":"modfact","versions":[{"version":"2.2.0"},{"version":"2.4.1"},{"version":"2.3.0"}]}`
	if err := os.WriteFile(filepath.Join(d, "versions.json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	manifestPath = filepath.Join(dir, "bindle.json")
	mf := `{"schema":"bindle/v0","name":"miapp","version":"0.1.0","private":true,"library":"MIAPP"}`
	if err := os.WriteFile(manifestPath, []byte(mf), 0o644); err != nil {
		t.Fatal(err)
	}
	return manifestPath, regDir
}

func TestAddResolvesLatest(t *testing.T) {
	mPath, regDir := setupAddProject(t)
	var buf bytes.Buffer
	if err := runAdd(&buf, mPath, regDir, []string{"modfact"}); err != nil {
		t.Fatalf("add: %v", err)
	}
	m, err := manifest.Load(mPath)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if got := m.Dependencies["modfact"]; got != "^2.4.1" {
		t.Errorf("constraint = %q, want ^2.4.1 (highest)", got)
	}
}

func TestAddExplicitConstraint(t *testing.T) {
	mPath, regDir := setupAddProject(t)
	var buf bytes.Buffer
	if err := runAdd(&buf, mPath, regDir, []string{"modfact@>=2.0.0 <3.0.0"}); err != nil {
		t.Fatalf("add: %v", err)
	}
	m, _ := manifest.Load(mPath)
	if got := m.Dependencies["modfact"]; got != ">=2.0.0 <3.0.0" {
		t.Errorf("constraint = %q", got)
	}
}

func TestAddInvalidConstraint(t *testing.T) {
	mPath, regDir := setupAddProject(t)
	var buf bytes.Buffer
	if err := runAdd(&buf, mPath, regDir, []string{"modfact@not-a-version"}); err == nil {
		t.Fatal("expected error for invalid constraint")
	}
}

func TestAddUnknownPackage(t *testing.T) {
	mPath, regDir := setupAddProject(t)
	var buf bytes.Buffer
	if err := runAdd(&buf, mPath, regDir, []string{"ghost"}); err == nil {
		t.Fatal("expected error for unknown package")
	}
}

func TestSplitSpec(t *testing.T) {
	cases := map[string][2]string{
		"modfact":          {"modfact", ""},
		"modfact@^2.3.0":   {"modfact", "^2.3.0"},
		"mod@>=1.0.0 <2.0": {"mod", ">=1.0.0 <2.0"},
	}
	for in, want := range cases {
		n, c := splitSpec(in)
		if n != want[0] || c != want[1] {
			t.Errorf("splitSpec(%q) = (%q,%q), want (%q,%q)", in, n, c, want[0], want[1])
		}
	}
}
