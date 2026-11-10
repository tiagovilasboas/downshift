// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package execwrap

// Output past the hold limit (compressor.MaxProcessableBytes). When the raw
// output can be kept for `downshift retrieve` (CCR on for every run and
// balanced compaction on), stdout is spooled to a temporary file instead of
// released: memory holds only its head and tail (spool.go). At exit the file
// is read back once and stored, and the run prints the head, the number of
// bytes cut with the retrieve id, the tail and the marker. Stderr passes
// straight through while spooling, so it is never held past the limit and
// never lost. When the output cannot be kept, it is released as before, with
// reason too_large.

// spoolable reports whether stdout past the hold limit may be spooled: only
// when every run's raw output is kept (Options.Keep), balanced compaction is
// on and the family may be cut with loss and stored at all.
func (g *gate) spoolable() bool {
	return g.keep != nil && g.balanced && !balanceRefused[g.family] && !rawStoreRefused[g.family]
}

// startSpool moves the held stdout to a new spool and writes the held stderr
// out, in its order. It reports false, leaving the held bytes as they were,
// when spooling is not allowed or the spool cannot be written. Callers hold
// g.mu.
func (g *gate) startSpool() bool {
	if !g.spoolable() {
		return false
	}
	s, err := newSpool()
	if err != nil {
		return false
	}
	ok := true
	g.eachHeld(func(isErr bool, b []byte) {
		if !isErr && ok && s.write(b) != nil {
			ok = false
		}
	})
	if !ok {
		s.discard()
		return false
	}
	g.eachHeld(func(isErr bool, b []byte) {
		if isErr {
			g.passed += int64(len(b))
			_, _ = g.err.Write(b)
		}
	})
	g.dropHeld()
	g.spool = s
	return true
}

// finishSpool ends a spooled run: the raw stdout is read back once and offered
// to the keep port; once stored, the run prints its view (spool.view). A read
// or store failure prints every spooled byte raw, reason too_large. The exit
// code is never changed. Callers hold g.mu.
func (g *gate) finishSpool(exitCode int, family Family) (raw, emitted int64, compacted, folded bool, passthrough string) {
	s := g.spool
	g.spool = nil
	defer s.discard()
	raw = g.nOut + g.nErr
	keep := g.keeper(exitCode, family)
	if data, err := s.readAll(); err == nil && keep != nil {
		if id, ok := keepSafely(keep, data); ok {
			view := s.view(id)
			_, _ = g.out.Write(view)
			return raw, int64(len(view)) + g.passed, true, true, ""
		}
	}
	s.replayTo(g.out)
	return raw, raw, false, false, PassthroughTooLarge
}

// keepSafely calls keep under recover: a panic counts as not stored.
func keepSafely(keep func(raw []byte) (string, bool), raw []byte) (id string, ok bool) {
	defer func() {
		if recover() != nil {
			id, ok = "", false
		}
	}()
	return keep(raw)
}
