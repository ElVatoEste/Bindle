// SPDX-License-Identifier: GPL-3.0-or-later

package builder

import (
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
