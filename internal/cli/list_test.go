// SPDX-License-Identifier: GPL-3.0-or-later

package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// setupProject writes a manifest + a local registry with modfact -> modbase,modimp.
func setupProject(t *testing.T) (manifestPath, regDir string) {
	t.Helper()
	dir := t.TempDir()
	regDir = filepath.Join(dir, "registry")

	writeReg := func(name, body string) {
		d := filepath.Join(regDir, name)
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(d, "versions.json"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	writeReg("modfact", `{"name":"modfact","versions":[
	  {"version":"2.3.0","dependencies":{"modbase":">=1.0.0 <2.0.0","modimp":"^1.2.0"}}]}`)
	writeReg("modbase", `{"name":"modbase","versions":[{"version":"1.4.2"}]}`)
	writeReg("modimp", `{"name":"modimp","versions":[{"version":"1.2.5"}]}`)

	manifestPath = filepath.Join(dir, "bindle.json")
	body := `{"schema":"bindle/v0","name":"miapp","version":"0.1.0","private":true,
	  "library":"MIAPP","dependencies":{"modfact":"^2.3.0"}}`
	if err := os.WriteFile(manifestPath, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return manifestPath, regDir
}

func TestRunListFlat(t *testing.T) {
	manifestPath, regDir := setupProject(t)

	var buf bytes.Buffer
	if err := runList(&buf, manifestPath, regDir, false); err != nil {
		t.Fatalf("runList: %v", err)
	}
	out := buf.String()
	for _, want := range []string{"miapp 0.1.0", "resolved 3 package(s)", "modbase", "1.4.2", "modfact", "2.3.0"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}

func TestRunListTree(t *testing.T) {
	manifestPath, regDir := setupProject(t)

	var buf bytes.Buffer
	if err := runList(&buf, manifestPath, regDir, true); err != nil {
		t.Fatalf("runList tree: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "└── modfact 2.3.0") {
		t.Errorf("tree missing modfact node:\n%s", out)
	}
	// modfact's children indented under it
	if !strings.Contains(out, "modbase 1.4.2") || !strings.Contains(out, "modimp 1.2.5") {
		t.Errorf("tree missing children:\n%s", out)
	}
}

func TestRunListNoRegistry(t *testing.T) {
	dir := t.TempDir()
	manifestPath := filepath.Join(dir, "bindle.json")
	body := `{"schema":"bindle/v0","name":"miapp","version":"0.1.0","private":true,
	  "library":"MIAPP","dependencies":{"modfact":"^2.3.0"}}`
	if err := os.WriteFile(manifestPath, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := runList(&buf, manifestPath, "", false); err == nil {
		t.Fatal("expected error when no registry is configured")
	}
}
