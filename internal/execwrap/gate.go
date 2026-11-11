// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package execwrap

import (
	"bytes"
	"io"
	"sync"
	"time"

	"github.com/tiagovilasboas/downshift-ai/internal/compressor"
)

// HoldBound, when positive, bounds how long the output of an allowlisted
// family is held while it could still be compacted. Past it the held bytes are
// flushed and the rest streams (reason hold_timeout). By default it is 0
// (disabled), so batch commands like `go test` hold the buffer until process
// completion without arbitrary timeouts.
var HoldBound time.Duration

// gate serialises the program's two output streams into the caller's two
// writers so their relative order is preserved. Both streams are held, in
// write order, while compaction is still possible (gate_order.go). Over
// compressor.MaxProcessableBytes, stdout is spooled to disk when the raw
// output can be kept for retrieve (gate_spool.go); otherwise the held bytes
// are released in write order and from then on both streams pass straight
// through. HoldBound (when positive) and, with a terminal on stdin, the first
// stderr byte of a family outside stderrIsReport release the same way. Each
// release records its reason (gate.reason).
type gate struct {
	mu              sync.Mutex
	out, err        io.Writer
	buf             bytes.Buffer // held stdout; stderr too when merge is set
	errBuf          bytes.Buffer // held stderr when merge is not set
	order           []segment    // held writes in order (gate_order.go)
	merge           bool         // stderr is part of the held report (stderrIsReport)
	stderrReleases  bool         // a stderr byte releases (interactive stdin)
	streaming       bool
	reason          string // why the gate released: a Passthrough* reason
	noteBytes       int    // tail notes a compacted output gets (run_tail.go)
	spool           *spool // stdout past the hold limit, nil unless spooling
	passed          int64  // stderr bytes passed through while spooling
	timer           *time.Timer
	nOut            int64
	nErr            int64
	argv            []string                        // for learning hook
	family          Family                          // for learning hook
	keep            func(raw []byte) (string, bool) // Options.Keep, nil when off
	keepFailed      func(raw []byte) (string, bool) // Options.KeepFailed
	detail          string                          // passthrough detail set by finish
	skipObservation bool
	desktop         func(view []byte) []byte // Options.Desktop
	balanced        bool                     // Options.Balanced
	bal             balanceConfig            // Options.BalanceThreshold, BalanceLimits
}

func newGate(out, err io.Writer, hold bool, argv []string, family Family) *gate {
	return &gate{out: out, err: err, streaming: !hold, merge: stderrIsReport[family], argv: argv, family: family}
}

// release flushes the held bytes in write order, or the spooled stdout, and
// switches to streaming. The first reason given is kept. Callers hold g.mu.
func (g *gate) release(reason string) {
	if g.streaming {
		return
	}
	g.streaming = true
	if g.reason == "" {
		g.reason = reason
	}
	if g.timer != nil {
		g.timer.Stop()
	}
	if g.spool != nil {
		g.spool.replayTo(g.out)
		g.spool.discard()
		g.spool = nil
		return
	}
	g.replay()
}

// overLimit handles held output that would pass the hold limit: stdout goes
// to a spool when its raw output can be kept, else everything is released.
// Callers hold g.mu.
func (g *gate) overLimit() {
	if !g.startSpool() {
		g.release(PassthroughTooLarge)
	}
}

type stdoutWriter struct{ g *gate }

func (w stdoutWriter) Write(p []byte) (int, error) {
	g := w.g
	g.mu.Lock()
	defer g.mu.Unlock()
	g.nOut += int64(len(p))
	if !g.streaming && g.spool == nil && g.heldLen()+len(p) > compressor.MaxProcessableBytes {
		g.overLimit()
	}
	switch {
	case g.streaming:
		_, _ = g.out.Write(p) // a closed reader must not kill the program
	case g.spool != nil:
		if err := g.spool.write(p); err != nil || g.spool.n > SpoolLimit {
			g.release(PassthroughTooLarge)
		}
	default:
		g.hold(p, false)
	}
	return len(p), nil
}

type stderrWriter struct{ g *gate }

func (w stderrWriter) Write(p []byte) (int, error) {
	g := w.g
	g.mu.Lock()
	defer g.mu.Unlock()
	g.nErr += int64(len(p))
	switch {
	case g.streaming:
	case g.spool != nil:
		g.passed += int64(len(p))
	case g.stderrReleases:
		g.release(PassthroughStderr)
	case g.heldLen()+len(p) > compressor.MaxProcessableBytes:
		g.overLimit()
		if g.spool != nil {
			g.passed += int64(len(p))
		}
	default:
		g.hold(p, true)
		return len(p), nil
	}
	_, _ = g.err.Write(p)
	return len(p), nil
}

// finish writes the still-held output, compacted (a failed run only for the
// formats CompressFailed supports) when nothing was released early and the
// compressor applied the expected format, raw otherwise, in write order. Held
// stderr of a family outside stderrIsReport is not compacted: it follows the
// compacted stdout whole, on the stderr writer. Spooled stdout is cut to its
// head and tail (finishSpool). It returns the raw and emitted byte counts,
// whether the compacted output was a lossy fold (raw kept in CCR) and, when
// the output was not compacted, the Passthrough reason. Output that streamed
// before the gate decided carries the reason the gate released it.
func (g *gate) finish(exitCode int, family Family) (raw, emitted int64, compacted, folded bool, passthrough string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	raw = g.nOut + g.nErr
	if g.streaming {
		passthrough = g.reason
		if passthrough == "" && exitCode != 0 {
			passthrough = PassthroughNonZeroExit
		}
		return raw, raw, false, false, passthrough
	}
	if g.timer != nil {
		g.timer.Stop()
	}
	g.streaming = true // a timer that fires after finish finds nothing to flush
	if g.spool != nil {
		return g.finishSpool(exitCode, family)
	}
	defer g.dropHeld()
	data, errHeld := g.buf.Bytes(), g.errBuf.Bytes()
	h := compactHeld(g.argv, data, family, exitCode, g.nErr > 0, g.keeper(exitCode, family), g.desktop, g.balanced, g.bal)
	out := h.out
	compacted, folded, passthrough, g.detail = h.compacted, h.folded, h.passthrough, h.detail
	if compacted && !folded && g.noteBytes > 0 && len(out)+g.noteBytes >= len(data) {
		// The notes Run appends count against the never-longer-than-raw rule.
		compacted, passthrough = false, PassthroughSmallOutput
	}
	if !compacted {
		g.replay()
		g.observe(data)
		return raw, raw, false, false, passthrough
	}
	_, _ = g.out.Write(out)
	_, _ = g.err.Write(errHeld)
	return raw, int64(len(out) + len(errHeld)), true, folded, ""
}

// observe runs the learning hook on output that was not compacted. The
// output is already written and the exit code already known, so the hook
// runs here, under a deadline, and its write is on disk when Run returns
// instead of being cut off by process exit.
func (g *gate) observe(data []byte) {
	if !g.skipObservation && ObserveUncompactedHook != nil && len(data) > 0 {
		observeUncompacted(ObserveUncompactedHook, g.argv, data, g.family)
	}
}

// ObserveHookDeadline bounds how long finish waits for ObserveUncompactedHook.
// A hook that overruns keeps running until the process exits; the command's
// output and exit code are never affected.
var ObserveHookDeadline = 500 * time.Millisecond

// observeUncompacted calls hook with a copy of data and waits for it until
// ObserveHookDeadline. A panic in the hook is swallowed.
func observeUncompacted(hook func([]string, []byte, Family), argv []string, data []byte, family Family) {
	sample := make([]byte, len(data))
	copy(sample, data)
	done := make(chan struct{})
	go func() {
		defer close(done)
		defer func() { recover() }() // never panic the caller
		hook(argv, sample, family)
	}()
	select {
	case <-done:
	case <-time.After(ObserveHookDeadline):
	}
}
