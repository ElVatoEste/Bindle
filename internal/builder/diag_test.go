// SPDX-License-Identifier: GPL-3.0-or-later

package builder

import (
	"strings"
	"testing"
)

func TestParseDiagnostics(t *testing.T) {
	out := `CRTRPGMOD ...
RNS9339: Unable to open file /home/x.rpgle.
RNS9309: Compilation failed. Module not created.
RNS9339: Unable to open file /home/x.rpgle.` // duplicate id ignored
	d := ParseDiagnostics(out)
	if len(d) != 2 {
		t.Fatalf("got %d diagnostics, want 2 (dedup): %+v", len(d), d)
	}
	if d[0].ID != "RNS9339" || !strings.Contains(d[0].Text, "Unable to open") {
		t.Errorf("first diag wrong: %+v", d[0])
	}
}

func TestParseDiagnosticsMatchesVariousIDs(t *testing.T) {
	for _, id := range []string{"CPF0006", "CPD0032", "MCH1202", "CPC5D0B", "RNF7030"} {
		d := ParseDiagnostics(id + ": some text")
		if len(d) != 1 || d[0].ID != id {
			t.Errorf("id %q not parsed: %+v", id, d)
		}
	}
}

func TestFormatDiagnosticsPrefersIDs(t *testing.T) {
	got := FormatDiagnostics("SAVOBJ", "", "CPF3770: No objects saved or restored.")
	if !strings.Contains(got, "SAVOBJ failed") || !strings.Contains(got, "CPF3770") {
		t.Errorf("format = %q", got)
	}
}

func TestFormatDiagnosticsFallbackRaw(t *testing.T) {
	// no recognizable message id -> raw output preserved
	got := FormatDiagnostics("CMD", "weird failure text", "")
	if !strings.Contains(got, "weird failure text") {
		t.Errorf("raw output not preserved: %q", got)
	}
}

func TestFormatDiagnosticsEmpty(t *testing.T) {
	got := FormatDiagnostics("CMD", "", "")
	if !strings.Contains(got, "no diagnostics") {
		t.Errorf("empty case = %q", got)
	}
}
