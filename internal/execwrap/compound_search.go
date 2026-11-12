// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package execwrap

import (
	"bytes"
	"strconv"
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

// leadingSegments drops the empty segments `&&` leaves and the directory
// moves (`cd dir && grep ...`, `pushd dir; rg ...`) that open a command, so the
// search behind them leads. The operator is not kept by compoundSegments, so
// `cd dir || grep` is skipped the same way, which only widens a cut that
// stores the raw output.
func leadingSegments(segs []string) []string {
	for len(segs) > 0 {
		_, argv := splitEnvAssignments(strings.Fields(segs[0]))
		if len(argv) > 0 && argv[0] != "cd" && argv[0] != "pushd" {
			break
		}
		segs = segs[1:]
	}
	return segs
}

// requestedLines returns the line count the last segment of command asks for
// with head or tail (`-n 50`, `-n50`, `-50`, `--lines=50`; none means the
// default 10) and true. A byte count, a `+N` offset or any other form is not
// a line limit and reports false.
func requestedLines(command string) (int, bool) {
	segs := compoundSegments(command)
	if len(segs) == 0 {
		return 0, false
	}
	seg := segs[len(segs)-1]
	if i := strings.IndexAny(seg, "<>"); i >= 0 {
		seg = seg[:i]
	}
	_, argv := splitEnvAssignments(strings.Fields(seg))
	if len(argv) == 0 || (argv[0] != "head" && argv[0] != "tail") {
		return 0, false
	}
	n := 10
	for i := 1; i < len(argv); i++ {
		a := argv[i]
		var val string
		switch {
		case a == "-n" || a == "--lines":
			if i+1 >= len(argv) {
				return 0, false
			}
			i++
			val = argv[i]
		case strings.HasPrefix(a, "--lines="):
			val = a[len("--lines="):]
		case strings.HasPrefix(a, "-n"):
			val = a[2:]
		case len(a) > 1 && a[0] == '-' && a[1] >= '0' && a[1] <= '9':
			val = a[1:]
		case strings.HasPrefix(a, "-"):
			return 0, false
		default:
			continue
		}
		v, err := strconv.Atoi(val)
		if err != nil || v < 0 || strings.HasPrefix(val, "+") {
			return 0, false
		}
		n = v
	}
	return n, true
}

// withinRequestedLimit reports whether data holds no more lines than the
// head or tail that ends command asked for: the agent chose those lines, so
// the cut leaves them. A limit larger than the output counts the same.
func withinRequestedLimit(command string, data []byte) bool {
	n, ok := requestedLines(command)
	if !ok {
		return false
	}
	lines := bytes.Count(data, []byte("\n"))
	if data[len(data)-1] != '\n' {
		lines++
	}
	return lines <= n
}

// LedBySearch reports whether the first program of command is a search: a
// file read (`cat f | grep -n x`) prints the file's own text, never rows.
func LedBySearch(command string) bool {
	segs := leadingSegments(compoundSegments(command))
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
// its marker is shorter than data, no trailing head or tail asked for at least
// as many lines as data holds, and keep stored the raw output. Otherwise
// it reports false and the caller keeps what it had. A panic reports false.
func CutSearch(command string, data []byte, exitCode int, keep func([]byte) (string, bool), session string) (res Captured, ok bool) {
	defer func() {
		if recover() != nil {
			res, ok = Captured{}, false
		}
	}()
	if keep == nil || exitCode > 1 || exitCode < 0 || len(data) == 0 || !LedBySearch(command) || withinRequestedLimit(command, data) {
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
