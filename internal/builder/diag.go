// SPDX-License-Identifier: GPL-3.0-or-later

package builder

import (
	"regexp"
	"strings"
)

// msgIDRe matches IBM i message identifiers: 3 letters + 4 hex/alnum, e.g.
// CPF0006, CPD0032, RNS9339, RNF7030, MCH1202, CPC5D0B.
var msgIDRe = regexp.MustCompile(`\b([A-Z]{2,3}[0-9][0-9A-Z]{3})\b`)

// Diagnostic is one IBM i message extracted from command output.
type Diagnostic struct {
	ID   string
	Text string
}

// ParseDiagnostics extracts distinct IBM i messages from command output
// (stdout+stderr of a `system "..."` call), preserving order. The text is the
// trimmed line the id appeared on.
func ParseDiagnostics(output string) []Diagnostic {
	var out []Diagnostic
	seen := map[string]bool{}
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		m := msgIDRe.FindString(line)
		if m == "" || seen[m] {
			continue
		}
		seen[m] = true
		out = append(out, Diagnostic{ID: m, Text: line})
	}
	return out
}

// FormatDiagnostics builds a readable failure message from a command's output.
// It surfaces the IBM i message ids found; if none, it falls back to the raw
// trimmed output so nothing is hidden.
func FormatDiagnostics(label, stdout, stderr string) string {
	combined := strings.TrimSpace(stderr)
	if combined == "" {
		combined = strings.TrimSpace(stdout)
	} else if s := strings.TrimSpace(stdout); s != "" {
		combined = combined + "\n" + s
	}

	diags := ParseDiagnostics(combined)
	if len(diags) == 0 {
		if combined == "" {
			return label + " failed (no diagnostics captured)"
		}
		return label + " failed:\n" + combined
	}

	var b strings.Builder
	b.WriteString(label + " failed:")
	for _, d := range diags {
		b.WriteString("\n  " + d.Text)
	}
	return b.String()
}
