// SPDX-License-Identifier: GPL-3.0-or-later

// Package ui renders Bindle's terminal output: colors and symbols when stdout is
// a TTY, plain ASCII when piped or when NO_COLOR is set. No heavy dependencies —
// ANSI escapes plus a small TTY probe.
package ui

import (
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/term"
)

// ANSI codes (empty when color is disabled).
const (
	cReset  = "\x1b[0m"
	cBold   = "\x1b[1m"
	cDim    = "\x1b[2m"
	cRed    = "\x1b[31m"
	cGreen  = "\x1b[32m"
	cYellow = "\x1b[33m"
	cBlue   = "\x1b[34m"
	cCyan   = "\x1b[36m"
	cGray   = "\x1b[90m"
)

// colorOn reports whether ANSI styling should be emitted to w.
//
// Off when: NO_COLOR is set, BINDLE_NO_COLOR is set, or w is not a terminal.
// On (forced) when BINDLE_FORCE_COLOR is set.
func colorOn(w io.Writer) bool {
	if os.Getenv("BINDLE_FORCE_COLOR") != "" {
		return true
	}
	if _, ok := os.LookupEnv("NO_COLOR"); ok {
		return false
	}
	if os.Getenv("BINDLE_NO_COLOR") != "" {
		return false
	}
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	return term.IsTerminal(int(f.Fd()))
}

// Printer renders styled output to a writer, downgrading to plain when needed.
type Printer struct {
	w     io.Writer
	color bool
}

// New returns a Printer for w with auto color detection.
func New(w io.Writer) *Printer { return &Printer{w: w, color: colorOn(w)} }

// Color reports whether this printer emits ANSI styling.
func (p *Printer) Color() bool { return p.color }

func (p *Printer) paint(code, s string) string {
	if !p.color {
		return s
	}
	return code + s + cReset
}

// Style helpers (no-op without color).
func (p *Printer) Bold(s string) string   { return p.paint(cBold, s) }
func (p *Printer) Dim(s string) string    { return p.paint(cDim, s) }
func (p *Printer) Red(s string) string    { return p.paint(cRed, s) }
func (p *Printer) Green(s string) string  { return p.paint(cGreen, s) }
func (p *Printer) Yellow(s string) string { return p.paint(cYellow, s) }
func (p *Printer) Blue(s string) string   { return p.paint(cBlue, s) }
func (p *Printer) Cyan(s string) string   { return p.paint(cCyan, s) }
func (p *Printer) Gray(s string) string   { return p.paint(cGray, s) }

// symbol returns a colored glyph (TTY) or an ASCII tag (plain).
func (p *Printer) symbol(glyph, tag, code string) string {
	if p.color {
		return code + glyph + cReset
	}
	return tag
}

// Status lines.

// OK prints a success line: "✓ msg".
func (p *Printer) OK(format string, a ...any) {
	fmt.Fprintln(p.w, p.symbol("✓", "[ok]", cGreen)+" "+fmt.Sprintf(format, a...))
}

// Fail prints an error line: "✗ msg".
func (p *Printer) Fail(format string, a ...any) {
	fmt.Fprintln(p.w, p.symbol("✗", "[fail]", cRed)+" "+fmt.Sprintf(format, a...))
}

// Warn prints a warning line: "! msg".
func (p *Printer) Warn(format string, a ...any) {
	fmt.Fprintln(p.w, p.symbol("!", "[warn]", cYellow)+" "+fmt.Sprintf(format, a...))
}

// Step prints a progress step: "→ msg" (dim arrow).
func (p *Printer) Step(format string, a ...any) {
	fmt.Fprintln(p.w, p.symbol("→", "->", cCyan)+" "+fmt.Sprintf(format, a...))
}

// Info prints a plain line.
func (p *Printer) Info(format string, a ...any) {
	fmt.Fprintln(p.w, fmt.Sprintf(format, a...))
}

// Printf writes without a trailing newline.
func (p *Printer) Printf(format string, a ...any) {
	fmt.Fprintf(p.w, format, a...)
}

// Writer exposes the underlying writer (for tabwriter, etc.).
func (p *Printer) Writer() io.Writer { return p.w }

// Heading prints a bold title.
func (p *Printer) Heading(format string, a ...any) {
	fmt.Fprintln(p.w, p.Bold(fmt.Sprintf(format, a...)))
}

// Bullet prints an indented gray bullet line.
func (p *Printer) Bullet(format string, a ...any) {
	fmt.Fprintln(p.w, "  "+p.Gray(fmt.Sprintf(format, a...)))
}

// KeyVal prints an aligned "key: value" line with a dim key.
func (p *Printer) KeyVal(key, val string) {
	fmt.Fprintf(p.w, "  %s %s\n", p.Dim(padRight(key+":", 11)), val)
}

func padRight(s string, n int) string {
	if len(s) >= n {
		return s
	}
	return s + strings.Repeat(" ", n-len(s))
}
