// SPDX-License-Identifier: GPL-3.0-or-later

package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/ElVatoEste/Bindle/internal/manifest"
)

func TestInitProject(t *testing.T) {
	file := filepath.Join(t.TempDir(), "bindle.json")
	var buf bytes.Buffer
	if err := runInit(&buf, initOptions{file: file, name: "miapp"}); err != nil {
		t.Fatalf("init: %v", err)
	}
	m, err := manifest.Load(file)
	if err != nil {
		t.Fatalf("generated manifest invalid: %v", err)
	}
	if !m.Private {
		t.Error("project should be private")
	}
	if m.Exports != nil {
		t.Error("project should have no exports")
	}
	if m.Registries["default"] == "" {
		t.Error("project should have a default registry")
	}
}

func TestInitModule(t *testing.T) {
	file := filepath.Join(t.TempDir(), "bindle.json")
	var buf bytes.Buffer
	if err := runInit(&buf, initOptions{file: file, name: "modfact", module: true}); err != nil {
		t.Fatalf("init: %v", err)
	}
	m, err := manifest.Load(file)
	if err != nil {
		t.Fatalf("generated manifest invalid: %v", err)
	}
	if m.Library != "MODFACT" {
		t.Errorf("library = %q, want MODFACT", m.Library)
	}
	if m.Exports == nil || m.Exports.Srvpgm == "" {
		t.Errorf("module should declare exports: %+v", m.Exports)
	}
	if !m.Publishable() {
		t.Error("module should be publishable")
	}
}

func TestInitRefusesOverwrite(t *testing.T) {
	file := filepath.Join(t.TempDir(), "bindle.json")
	if err := os.WriteFile(file, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := runInit(&buf, initOptions{file: file, name: "x"}); err == nil {
		t.Fatal("expected refusal to overwrite without --force")
	}
	if err := runInit(&buf, initOptions{file: file, name: "x", force: true}); err != nil {
		t.Errorf("with --force should overwrite: %v", err)
	}
}

func TestDeriveLibrary(t *testing.T) {
	cases := map[string]string{
		"modfact":      "MODFACT",
		"mod-clientes": "MODCLIENTE", // truncated to 10
		"123app":       "LIB123APP",  // must start with a letter
	}
	for in, want := range cases {
		if got := deriveLibrary(in); got != want {
			t.Errorf("deriveLibrary(%q) = %q, want %q", in, got, want)
		}
	}
}
