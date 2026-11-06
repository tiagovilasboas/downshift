// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package workers

import (
	"errors"
	"sync"
)

// ErrNoVerifiedMode is what the default launcher returns: no agent has a
// read-only launch that was checked against the real CLI yet (spec R27).
var ErrNoVerifiedMode = errors.New("no verified read-only launch for this agent yet (docs/internal/plans/mcp-delegation-spec.md, R27)")

// Process is one running worker.
type Process interface {
	// Wait blocks until the worker exits and returns a non-nil error when it
	// failed to finish.
	Wait() error
	// Kill stops the worker. It is safe to call after the worker has exited.
	Kill() error
	// Report is the output so far, bounded.
	Report() string
	// Stderr is the bounded tail of the worker's standard error.
	Stderr() string
	// Tokens is the usage the worker reported once it exited; nil when unknown.
	Tokens() *Tokens
}

// Launcher starts a worker for a spec.
type Launcher interface {
	Launch(spec Spec) (Process, error)
}

// maxOutput bounds what a job keeps of its own output, in bytes (spec 5.2 caps
// the live reply at 16 000 bytes; a delivery is cut to the same bound).
const maxOutput = 16_000

// Tail keeps the last max bytes written to it. It is safe for concurrent use,
// so a worker's output reader and the queue can share it.
type Tail struct {
	mu  sync.Mutex
	b   []byte
	max int
}

// NewTail returns a Tail that keeps at most max bytes.
func NewTail(max int) *Tail { return &Tail{max: max} }

// Write appends p and drops the oldest bytes past the bound.
func (t *Tail) Write(p []byte) (int, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.b = append(t.b, p...)
	if over := len(t.b) - t.max; over > 0 {
		t.b = append([]byte(nil), t.b[over:]...)
	}
	return len(p), nil
}

// String returns the kept bytes.
func (t *Tail) String() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return string(t.b)
}

// Head keeps the first bytes written to it, up to a bound, and drops the rest.
// It is the counterpart of Tail for output whose identity line comes first: a
// CLI that prints its session id at the start of a long event stream would
// otherwise lose that line once the tail rolls past it.
type Head struct {
	mu  sync.Mutex
	b   []byte
	max int
}

// NewHead returns a Head that keeps at most max bytes.
func NewHead(max int) *Head { return &Head{max: max} }

// Write keeps p up to the bound and reports every byte as written, so it never
// stalls the writer it is paired with.
func (h *Head) Write(p []byte) (int, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if room := h.max - len(h.b); room > 0 {
		if len(p) < room {
			room = len(p)
		}
		h.b = append(h.b, p[:room]...)
	}
	return len(p), nil
}

// Bytes returns a copy of the kept bytes.
func (h *Head) Bytes() []byte {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]byte(nil), h.b...)
}

// unverified refuses every launch: the real CLIs have not been checked for a
// read-only run, and a worker is never started on a guess.
type unverified struct{}

// Unverified returns the default launcher.
func Unverified() Launcher { return unverified{} }

// Launch always fails with ErrNoVerifiedMode.
func (unverified) Launch(Spec) (Process, error) { return nil, ErrNoVerifiedMode }
