// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package execwrap

import (
	"bytes"
	"time"
)

// How the gate holds both streams in the order the program wrote them. The
// bytes live once: stdout (and, for a family in stderrIsReport, stderr) in
// gate.buf, other stderr in gate.errBuf. gate.order records which stream each
// run of consecutive writes went to and how long it was, so a replay writes
// each byte to its own stream in write order without a second copy.

// segment is a run of consecutive held bytes written to one stream.
type segment struct {
	isErr bool
	n     int
}

// hold appends a write to the held bytes and starts the HoldBound timer on
// the first one. Callers hold g.mu.
func (g *gate) hold(p []byte, isErr bool) {
	if g.timer == nil && HoldBound > 0 {
		g.timer = time.AfterFunc(HoldBound, func() {
			g.mu.Lock()
			defer g.mu.Unlock()
			g.release(PassthroughHoldTimeout)
		})
	}
	if isErr && !g.merge {
		g.errBuf.Write(p)
	} else {
		g.buf.Write(p)
	}
	if k := len(g.order); k > 0 && g.order[k-1].isErr == isErr {
		g.order[k-1].n += len(p)
		return
	}
	g.order = append(g.order, segment{isErr, len(p)})
}

// heldLen is the number of bytes held in memory.
func (g *gate) heldLen() int { return g.buf.Len() + g.errBuf.Len() }

// eachHeld calls fn for each held segment, in write order.
func (g *gate) eachHeld(fn func(isErr bool, b []byte)) {
	out, errb := g.buf.Bytes(), g.errBuf.Bytes()
	for _, s := range g.order {
		var b []byte
		if s.isErr && !g.merge {
			b, errb = errb[:s.n], errb[s.n:]
		} else {
			b, out = out[:s.n], out[s.n:]
		}
		fn(s.isErr, b)
	}
}

// replay writes the held bytes raw, each to the stream it came from, in
// write order, and drops them. Callers hold g.mu.
func (g *gate) replay() {
	g.eachHeld(func(isErr bool, b []byte) {
		if isErr {
			_, _ = g.err.Write(b) // a closed reader must not kill the program
		} else {
			_, _ = g.out.Write(b)
		}
	})
	g.dropHeld()
}

// dropHeld forgets the held bytes.
func (g *gate) dropHeld() {
	g.buf, g.errBuf, g.order = bytes.Buffer{}, bytes.Buffer{}, nil
}
