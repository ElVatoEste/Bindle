// SPDX-License-Identifier: GPL-3.0-or-later

package ui

import (
	"fmt"
	"sync"
	"time"
)

var spinFrames = []rune{'⠋', '⠙', '⠹', '⠸', '⠼', '⠴', '⠦', '⠧', '⠇', '⠏'}

// Spinner is a single-line progress indicator. On a TTY it animates in place; in
// plain mode it degrades to one "→ label" line per Start/Update (no animation,
// pipe-safe).
type Spinner struct {
	p      *Printer
	mu     sync.Mutex
	label  string
	stop   chan struct{}
	done   chan struct{}
	active bool
}

// Spinner returns a new spinner bound to this printer.
func (p *Printer) Spinner(label string) *Spinner {
	return &Spinner{p: p, label: label}
}

// Start begins the spinner. Safe to call once.
func (s *Spinner) Start() {
	if !s.p.color {
		s.p.Step("%s", s.label) // plain mode: just announce the step
		return
	}
	s.active = true
	s.stop = make(chan struct{})
	s.done = make(chan struct{})
	go s.run()
}

func (s *Spinner) run() {
	defer close(s.done)
	t := time.NewTicker(90 * time.Millisecond)
	defer t.Stop()
	i := 0
	for {
		select {
		case <-s.stop:
			return
		case <-t.C:
			s.mu.Lock()
			frame := string(spinFrames[i%len(spinFrames)])
			fmt.Fprintf(s.p.w, "\r%s %s ", s.p.Cyan(frame), s.label)
			s.mu.Unlock()
			i++
		}
	}
}

// Update changes the label mid-spin.
func (s *Spinner) Update(format string, a ...any) {
	label := fmt.Sprintf(format, a...)
	if !s.p.color {
		s.p.Step("%s", label)
		return
	}
	s.mu.Lock()
	s.label = label
	s.mu.Unlock()
}

// Stop ends the spinner, clearing the line (TTY) before the caller prints a final
// status (e.g. p.OK).
func (s *Spinner) Stop() {
	if !s.p.color || !s.active {
		return
	}
	close(s.stop)
	<-s.done
	s.active = false
	// clear the spinner line
	fmt.Fprintf(s.p.w, "\r\x1b[K")
}
