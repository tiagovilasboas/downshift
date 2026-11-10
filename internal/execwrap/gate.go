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

// HoldBound, when positive, bounds how long stdout of an allowlisted family
// is held while it could still be compacted. Past it the held bytes are flushed
// and the rest streams. By default it is 0 (disabled), so batch commands like
// `go test` hold the buffer until process completion without arbitrary timeouts.
var HoldBound time.Duration

// gate serialises the program's two output streams into the caller's two
// writers so their relative order is preserved. Stdout is held back only
// while compaction is still possible: the first stderr byte, the size limit
// or HoldBound (when positive) releases the held bytes first, in write order,
// and from then on both streams pass straight through and nothing is compacted.
type gate struct {
	mu              sync.Mutex
	out, err        io.Writer
	buf             bytes.Buffer
	merge           bool    // stderr is part of the held report (stderrIsReport)
	chunks          []chunk // held writes in order, kept only when merge is set
	streaming       bool
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
}

func newGate(out, err io.Writer, hold bool, argv []string, family Family) *gate {
	return &gate{out: out, err: err, streaming: !hold, merge: stderrIsReport[family], argv: argv, family: family}
}

// chunk is one held write and the stream it was written to.
type chunk struct {
	isErr bool
	b     []byte
}

// replay writes the held bytes raw, each to the stream it came from.
func (g *gate) replay(data []byte) {
	if !g.merge {
		_, _ = g.out.Write(data) // a closed reader must not kill the program
		return
	}
	for _, c := range g.chunks {
		if c.isErr {
			_, _ = g.err.Write(c.b)
		} else {
			_, _ = g.out.Write(c.b)
		}
	}
	g.chunks = nil
}

// hold appends a write to the held buffer. Callers hold g.mu.
func (g *gate) hold(p []byte, isErr bool) {
	g.buf.Write(p)
	if g.merge {
		g.chunks = append(g.chunks, chunk{isErr, append([]byte(nil), p...)})
	}
}

// release flushes held stdout and switches to streaming. Callers hold g.mu.
func (g *gate) release() {
	if g.streaming {
		return
	}
	g.streaming = true
	if g.timer != nil {
		g.timer.Stop()
	}
	g.replay(g.buf.Bytes())
	g.buf = bytes.Buffer{}
}

type stdoutWriter struct{ g *gate }

func (w stdoutWriter) Write(p []byte) (int, error) {
	g := w.g
	g.mu.Lock()
	defer g.mu.Unlock()
	g.nOut += int64(len(p))
	if !g.streaming && g.buf.Len()+len(p) > compressor.MaxProcessableBytes {
		g.release()
	}
	if g.streaming {
		_, _ = g.out.Write(p)
		return len(p), nil
	}
	if g.buf.Len() == 0 && HoldBound > 0 {
		g.timer = time.AfterFunc(HoldBound, func() {
			g.mu.Lock()
			defer g.mu.Unlock()
			g.release()
		})
	}
	g.hold(p, false)
	return len(p), nil
}

type stderrWriter struct{ g *gate }

func (w stderrWriter) Write(p []byte) (int, error) {
	g := w.g
	g.mu.Lock()
	defer g.mu.Unlock()
	g.nErr += int64(len(p))
	if g.merge && !g.streaming {
		if g.buf.Len()+len(p) > compressor.MaxProcessableBytes {
			g.release()
		} else {
			g.hold(p, true)
			return len(p), nil
		}
	}
	g.release()
	_, _ = g.err.Write(p)
	return len(p), nil
}

// finish writes any still-held stdout, compacted (a failed run only for the
// formats CompressFailed supports),
// nothing was released early and the compressor applied the expected format,
// raw otherwise. It returns the raw and emitted byte counts, whether the compacted
// output was a lossy fold (raw kept in CCR) and, when the
// output was not compacted, the Passthrough reason. Output that streamed
// before the gate decided has no reason beyond a non-zero exit.
func (g *gate) finish(exitCode int, family Family) (raw, emitted int64, compacted, folded bool, passthrough string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	raw = g.nOut + g.nErr
	if g.streaming {
		if exitCode != 0 {
			passthrough = PassthroughNonZeroExit
		}
		return raw, raw, false, false, passthrough
	}
	if g.timer != nil {
		g.timer.Stop()
	}
	data := g.buf.Bytes()
	g.streaming = true // a timer that fires after finish finds nothing to flush
	g.buf = bytes.Buffer{}
	var out []byte
	out, passthrough, g.detail = compactOutputFor(g.argv, exitCode, data, family, g.nErr > 0)
	compacted = passthrough == ""
	if compacted {
		out = keepRawFor(family, g.keeper(exitCode, family), data, out)
		out = applyDesktop(g.desktop, family, data, out)
	} else if foldedOut, ok := keepUncompacted(g.keeper(exitCode, family), data, exitCode, family, g.nErr > 0); ok {
		out, compacted, folded, passthrough, g.detail = foldedOut, true, true, "", ""
	}
	if g.balanced {
		shown := data
		if compacted {
			shown = out
		}
		if cut, ok := balance(g.keeper(exitCode, family), family, data, shown); ok {
			out, compacted, folded, passthrough, g.detail = cut, true, true, "", ""
		}
	}
	if compacted {
		_, _ = g.out.Write(out)
	} else {
		g.replay(data)
	}

	// The output is already written and the exit code already known, so the
	// learning hook runs here, under a deadline, and its write is on disk
	// when Run returns instead of being cut off by process exit.
	if !g.skipObservation && !compacted && ObserveUncompactedHook != nil && len(data) > 0 {
		observeUncompacted(ObserveUncompactedHook, g.argv, data, g.family)
	}

	return raw, int64(len(out)), compacted, folded, passthrough
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

// keeper picks the storage port for a finished run: Keep for every run, else
// KeepFailed for a run that exited non-zero. A family in rawStoreRefused (an
// HTTP response may carry Set-Cookie) gets no storage port at all.
func (g *gate) keeper(exitCode int, family Family) func(raw []byte) (string, bool) {
	if rawStoreRefused[family] {
		return nil
	}
	if g.keep != nil || exitCode == 0 {
		return g.keep
	}
	return g.keepFailed
}
