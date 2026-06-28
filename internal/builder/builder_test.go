// SPDX-License-Identifier: GPL-3.0-or-later

package builder

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ElVatoEste/Bindle/internal/manifest"
)

var fakeManifest = manifest.Manifest{
	Build: &manifest.Build{Objects: []string{"GREETMOD"}},
}

func TestParseSignature(t *testing.T) {
	out := `                              Display Service Program Information
 Service program . . . . :   GREETSRV
 Detail  . . . . . . . . :   *SIGNATURE
                                       Signatures:
 0000000000000000000000E3C5C5D9C7
                           * * * * *   E N D   O F   L I S T I N G
`
	sig, err := parseSignature(out)
	if err != nil {
		t.Fatalf("parseSignature: %v", err)
	}
	if sig != "0000000000000000000000E3C5C5D9C7" {
		t.Errorf("sig = %q", sig)
	}
}

func TestParseSignatureMultiple(t *testing.T) {
	// when several signatures are listed, the first (current) is returned
	out := `Signatures:
 ABCDEF0123456789ABCDEF0123456789
 0000000000000000000000E3C5C5D9C7
`
	sig, err := parseSignature(out)
	if err != nil {
		t.Fatalf("parseSignature: %v", err)
	}
	if sig != "ABCDEF0123456789ABCDEF0123456789" {
		t.Errorf("expected first signature, got %q", sig)
	}
}

func TestParseSignatureNone(t *testing.T) {
	if _, err := parseSignature("no signatures here\n"); err == nil {
		t.Error("expected error when no signature present")
	}
}

func TestModulesOf(t *testing.T) {
	if got := modulesOf(&fakeManifest); len(got) != 1 || got[0] != "GREETMOD" {
		t.Errorf("modulesOf = %v", got)
	}
}

func TestDeterministicSignature(t *testing.T) {
	// stable across minor/patch (same major), distinct across major
	s1 := DeterministicSignature("modgreet", 0)
	s2 := DeterministicSignature("modgreet", 0)
	s3 := DeterministicSignature("modgreet", 1)
	if s1 != s2 {
		t.Errorf("same name+major should be stable: %s != %s", s1, s2)
	}
	if s1 == s3 {
		t.Error("major bump should change the signature")
	}
	if len(s1) != 32 {
		t.Errorf("signature len = %d, want 32 hex chars", len(s1))
	}
}

func TestBinderSource(t *testing.T) {
	got := binderSource("ABCD", []string{"GREET", "BYE"})
	for _, want := range []string{
		"STRPGMEXP PGMLVL(*CURRENT) SIGNATURE(X'ABCD')",
		"EXPORT SYMBOL('GREET')",
		"EXPORT SYMBOL('BYE')",
		"ENDPGMEXP",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("binder source missing %q:\n%s", want, got)
		}
	}
}

func TestExportSymbolsScan(t *testing.T) {
	dir := t.TempDir()
	src := "**free\nctl-opt nomain;\ndcl-proc bgreet export;\n  dcl-pi *n char(1); end-pi;\nend-proc;\n" +
		"dcl-proc helper;\nend-proc;\n" // not exported
	if err := os.WriteFile(filepath.Join(dir, "GREET.rpgle"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	m := &manifest.Manifest{Build: &manifest.Build{Objects: []string{"GREET"}}}
	got, err := exportSymbols(m, dir, []string{"GREET"})
	if err != nil {
		t.Fatalf("exportSymbols: %v", err)
	}
	if len(got) != 1 || got[0] != "BGREET" {
		t.Errorf("exports = %v, want [BGREET]", got)
	}
}

func TestExportSymbolsManifestOverride(t *testing.T) {
	m := &manifest.Manifest{Exports: &manifest.Exports{Symbols: []string{"foo", "bar"}}}
	got, err := exportSymbols(m, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0] != "FOO" || got[1] != "BAR" {
		t.Errorf("exports = %v, want [FOO BAR]", got)
	}
}
