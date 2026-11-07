// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package execwrap

import (
	"bytes"
	"fmt"

	"github.com/tiagovilasboas/downshift/internal/compressor"
)

// markerPrefix starts the marker line compactSafely appends to a summary.
const markerPrefix = "[downshift compact: "

// MarkerWithID is the marker of a compacted run whose raw output was kept: it
// tells the reader which id to pass to `downshift retrieve`.
func MarkerWithID(raw, summary int, id string) string {
	return fmt.Sprintf("[downshift compact: compacted %d to %d bytes; full output: downshift retrieve %s]\n", raw, summary, id)
}

// keepRaw offers the raw held bytes to keep (Options.Keep) and, when it
// stores them, swaps the trailing marker of out for MarkerWithID. Keep never
// fails the run: a panic, a false return or an out without a marker leaves out
// unchanged.
func keepRaw(keep func(raw []byte) (string, bool), raw, out []byte) (res []byte) {
	res = out
	defer func() {
		if recover() != nil {
			res = out
		}
	}()
	if keep == nil {
		return out
	}
	i := bytes.LastIndex(out, []byte(markerPrefix))
	if i < 0 {
		return out
	}
	summary := foldStored(out[:i])
	if len(raw) > len(out) && len(summary)+len(MarkerWithID(len(raw), len(summary), "000000000000")) >= len(raw) {
		return out
	}
	id, ok := keep(append([]byte(nil), raw...))
	if !ok {
		return out
	}
	withID := append(append([]byte{}, summary...), MarkerWithID(len(raw), len(summary), id)...)
	if len(raw) > len(out) && len(withID) >= len(raw) {
		return out
	}
	return withID
}

// foldStored cuts runs of same-shape lines (compressor.FoldRepeats) from text
// whose raw output is already stored, so the cut lines stay retrievable. Text
// it cannot shorten comes back unchanged.
func foldStored(text []byte) []byte {
	r := compressor.FoldRepeats(text)
	if !r.Applied || len(r.Output) >= len(text) {
		return text
	}
	return r.Output
}

// keepUncompacted handles a clean run of an allowlisted family that the
// compressor chain kept raw: with Keep set, its same-shape line runs are
// folded once the raw output is stored. It reports false, and the caller
// replays the raw output, when the run failed, wrote to stderr, has nothing
// to fold or could not be stored. A panic in Keep counts as false. A file read
// is never folded this way: its lines are the file's, not a log's repeats.
// File listings under 100 lines (find, ls) are also preserved without folding.
func keepUncompacted(keep func(raw []byte) (string, bool), raw []byte, exitCode int, family Family, wroteStderr bool) (res []byte, ok bool) {
	defer func() {
		if recover() != nil {
			res, ok = nil, false
		}
	}()
	if keep == nil || exitCode != 0 || wroteStderr || !compactibleFamily(family) || family == FamilyFileRead {
		return nil, false
	}
	if (family == FamilyFind || family == FamilyLs) && bytes.Count(raw, []byte("\n")) <= 100 {
		return nil, false
	}
	summary := foldStored(raw)
	if len(summary) == len(raw) || len(summary)+len(MarkerWithID(len(raw), len(summary), "000000000000")) >= len(raw) {
		return nil, false
	}
	id, stored := keep(append([]byte(nil), raw...))
	if !stored {
		return nil, false
	}
	return append(append([]byte{}, summary...), MarkerWithID(len(raw), len(summary), id)...), true
}
