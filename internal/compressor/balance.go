// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/tiagovilasboas/downshift-ai/internal/lazyre"
)

// Balanced compaction: a lossy cut of a large output whose raw text is
// already stored (CCR), so every line it drops can be read back with
// `downshift retrieve`. The caller only calls it when the raw output was
// stored; these functions never decide that.
//
// The cut keeps what an agent acts on first:
//   - a diff keeps every file header, every hunk header and the first lines
//     of each hunk, and says how many lines of each hunk it left out;
//   - a list of matches (grep, rg, git grep) keeps the first matches of each
//     file and a count of the rest, and cuts very long lines;
//   - any other output keeps its head, its tail and every line with a
//     failure word (error, fail, panic, ...) in between.
//
// The log cut keeps up to balanceLogKeep failure lines between the head and
// the tail; a run with more says how many lines it cut, and all are stored.

// BalanceKind is the shape of the output being cut.
type BalanceKind int

const (
	// BalanceLog is any output read top to bottom: test runs, builds, pushes.
	BalanceLog BalanceKind = iota
	// BalanceDiff is a unified diff or a git show transcript.
	BalanceDiff
	// BalanceMatches is one match per line, "path:..." (grep, rg, git grep).
	BalanceMatches
)

// Default limits of the balanced cut, in lines (BalanceLimits overrides the
// first six), and the fixed line and failure caps.
const (
	balanceHunkLines    = 30  // body lines shown per diff hunk
	balanceDiffLines    = 250 // body lines shown across a diff
	balanceMatchPerFile = 20  // matches shown per file
	balanceMatchLines   = 250 // matches shown in total
	balanceLineCells    = 300 // a longer line is cut
	balanceLineTail     = 200 // bytes a cut failure line keeps from its end
	balanceLogHead      = 60  // first lines of a log
	balanceLogTail      = 80  // last lines of a log
	balanceLogKeep      = 300 // failure lines kept between head and tail
	balanceLogAfter     = 3   // lines kept after each failure line
)

// Balance cuts text of kind with the default limits. It reports false when
// the cut would not save at least a quarter of the text, and the caller keeps
// what it had.
func Balance(kind BalanceKind, text []byte) ([]byte, bool) {
	return BalanceWith(kind, text, BalanceLimits{})
}

// BalanceWith is Balance with lim; a zero field of lim keeps its default.
func BalanceWith(kind BalanceKind, text []byte, lim BalanceLimits) ([]byte, bool) {
	lim = lim.WithDefaults()
	lines := strings.Split(strings.TrimSuffix(string(text), "\n"), "\n")
	var out []string
	switch kind {
	case BalanceDiff:
		out = balanceDiff(lines, lim)
	case BalanceMatches:
		out = balanceMatches(lines, lim)
	default:
		out = balanceLog(lines, lim)
	}
	res := []byte(strings.Join(out, "\n") + "\n")
	if len(res)*4 > len(text)*3 {
		return text, false
	}
	return res, true
}

// balanceCut shortens a line longer than balanceLineCells. A failure line
// (one the log cut keeps for its words) keeps its last balanceLineTail bytes
// too: an assertion often ends the line.
func balanceCut(l string) string {
	if len(l) <= balanceLineCells {
		return l
	}
	n := balanceLineCells
	for n > 0 && !utf8.RuneStart(l[n]) {
		n-- // never split a multi-byte character
	}
	if len(l) > balanceLineCells+balanceLineTail && reKeepLine.MatchString(l) {
		m := len(l) - balanceLineTail
		for m < len(l) && !utf8.RuneStart(l[m]) {
			m++
		}
		return l[:n] + fmt.Sprintf(" … (%d bytes cut) … ", m-n) + l[m:]
	}
	return l[:n] + fmt.Sprintf(" … (%d more bytes)", len(l)-n)
}

// balanceDiff keeps headers and the first lines of each hunk.
func balanceDiff(lines []string, lim BalanceLimits) []string {
	var out []string
	shown, inHunk, hunkShown, hunkCut, total := 0, false, 0, 0, 0
	flush := func() {
		if hunkCut > 0 {
			out = append(out, fmt.Sprintf("… %d more lines in this hunk (downshift retrieve)", hunkCut))
		}
		hunkCut = 0
	}
	for _, l := range lines {
		switch {
		case strings.HasPrefix(l, "diff --git ") || strings.HasPrefix(l, "commit "):
			flush()
			inHunk = false
			out = append(out, l)
		case strings.HasPrefix(l, "@@"):
			flush()
			inHunk, hunkShown = true, 0
			out = append(out, l)
		case !inHunk:
			out = append(out, balanceCut(l))
		case hunkShown < lim.HunkLines && shown < lim.DiffLines:
			hunkShown++
			shown++
			out = append(out, balanceCut(l))
		default:
			hunkCut++
			total++
		}
	}
	flush()
	if total > 0 {
		out = append(out, fmt.Sprintf("… %d diff lines cut in all; every file and hunk header is above", total))
	}
	return out
}

// matchFile is the file part of a "path:line:text" or "path:text" match.
func matchFile(l string) string {
	if i := strings.IndexByte(l, ':'); i > 0 {
		return l[:i]
	}
	return ""
}

// balanceMatches keeps the first matches of each file and counts the rest.
func balanceMatches(lines []string, lim BalanceLimits) []string {
	var out []string
	perFile := map[string]int{}
	cutFiles := map[string]int{}
	var order []string
	shown := 0
	for _, l := range lines {
		f := matchFile(l)
		if perFile[f] < lim.MatchesPerFile && shown < lim.MatchLines {
			perFile[f]++
			shown++
			out = append(out, balanceCut(l))
			continue
		}
		if cutFiles[f] == 0 {
			order = append(order, f)
		}
		cutFiles[f]++
	}
	if len(order) == 0 {
		return out
	}
	cut := 0
	for _, f := range order {
		cut += cutFiles[f]
	}
	out = append(out, fmt.Sprintf("… %d more matches in %d files (downshift retrieve):", cut, len(order)))
	for i, f := range order {
		if i == 30 {
			out = append(out, fmt.Sprintf("  … and %d more files", len(order)-30))
			break
		}
		out = append(out, fmt.Sprintf("  %s: %d more", f, cutFiles[f]))
	}
	return out
}

// reBalanceScaffold is a test-runner progress line whose test name may hold a
// failure word ("=== RUN TestX/error"): it is not a failure, so the log cut
// does not keep it for that word.
var reBalanceScaffold = lazyre.New(`^\s*(=== (RUN|PAUSE|CONT|NAME)|--- (PASS|SKIP)|PASS$|ok\s)`)

// balanceLog keeps the head, the tail and every failure line between them.
func balanceLog(lines []string, lim BalanceLimits) []string {
	if len(lines) <= lim.LogHead+lim.LogTail {
		out := make([]string, len(lines))
		for i, l := range lines {
			out[i] = balanceCut(l)
		}
		return out
	}
	var out []string
	for _, l := range lines[:lim.LogHead] {
		out = append(out, balanceCut(l))
	}
	mid := lines[lim.LogHead : len(lines)-lim.LogTail]
	kept, gap, after := 0, 0, 0
	for _, l := range mid {
		failure := reKeepLine.MatchString(l) && !reBalanceScaffold.MatchString(l)
		if failure {
			after = balanceLogAfter
		} else if after > 0 {
			after-- // the lines right after a failure: its assertion, its location
			failure = true
		}
		if kept < balanceLogKeep && failure {
			if gap > 0 {
				out = append(out, fmt.Sprintf("… %d lines cut (downshift retrieve)", gap))
				gap = 0
			}
			kept++
			out = append(out, balanceCut(l))
			continue
		}
		gap++
	}
	if gap > 0 {
		out = append(out, fmt.Sprintf("… %d lines cut (downshift retrieve)", gap))
	}
	for _, l := range lines[len(lines)-lim.LogTail:] {
		out = append(out, balanceCut(l))
	}
	return out
}
