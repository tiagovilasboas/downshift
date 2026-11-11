// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package execwrap

import (
	"strings"

	"github.com/tiagovilasboas/downshift-ai/internal/compressor"
)

// A command led by a search that no family covers (`grep -rn x . | head`,
// `git grep -n x | grep -v y`, an rg form the policy does not accept) prints
// search rows. No compressor reads it; under BalanceThreshold the balanced
// cut leaves it whole, and above it the log cut reads it as a log.
// CutSearch keeps the first rows of each file instead
// (compressor.CapSearchRows), with the raw output stored so every dropped
// row comes back with `downshift retrieve`.

// searchPrograms lead a command whose output CutSearch may read.
var searchPrograms = map[string]bool{"grep": true, "egrep": true, "fgrep": true, "rg": true, "ag": true, "ack": true}

// LedBySearch reports whether the first program of command is a search: a
// file read (`cat f | grep -n x`) prints the file's own text, never rows.
func LedBySearch(command string) bool {
	segs := compoundSegments(command)
	if len(segs) == 0 {
		return false
	}
	seg := segs[0]
	if i := strings.IndexAny(seg, "<>"); i >= 0 {
		seg = seg[:i]
	}
	_, argv := splitEnvAssignments(strings.Fields(seg))
	if len(argv) == 0 {
		return false
	}
	if searchPrograms[argv[0]] {
		return true
	}
	return argv[0] == "git" && len(argv) > 1 && argv[1] == "grep"
}

// CutSearch cuts data, the stdout of a command with no family, when the
// command is led by a search, the run did not fail (exit 0, or 1 for "no
// match" from a filter), compressor.CapSearchRows drops a row, the cut with
// its marker is shorter than data, and keep stored the raw output. Otherwise
// it reports false and the caller keeps what it had. A panic reports false.
func CutSearch(command string, data []byte, exitCode int, keep func([]byte) (string, bool), session string) (res Captured, ok bool) {
	defer func() {
		if recover() != nil {
			res, ok = Captured{}, false
		}
	}()
	if keep == nil || exitCode > 1 || exitCode < 0 || len(data) == 0 || !LedBySearch(command) {
		return Captured{}, false
	}
	body, applied := compressor.CapSearchRows(data)
	if !applied || len(body)+len(MarkerWithID(len(data), len(body), longestHintID)) >= len(data) {
		return Captured{}, false
	}
	var kept string
	id, stored := captureKept(keep, &kept, session)(append([]byte(nil), data...))
	if !stored {
		return Captured{}, false
	}
	out := append(body, MarkerWithID(len(data), len(body), id)...)
	return Captured{Output: out, Compacted: true, Folded: true, KeptID: kept}, true
}
