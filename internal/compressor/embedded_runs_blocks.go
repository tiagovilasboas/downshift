// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import (
	"strings"

	"github.com/tiagovilasboas/downshift-ai/internal/lazyre"
)

var (
	rePytestSessionLine = lazyre.New(`^=+ test session starts =+$`)
	reDroppedMarker     = lazyre.New(`^\[\d+ console output lines dropped\]$`)
)

func blockText(lines []string) []byte { return []byte(strings.Join(lines, "\n") + "\n") }

// goVerboseBlock bounds a `go test -v` package: from its first "=== RUN" line
// to the package's "ok" line. The block folds only when every line is a test
// run, outcome, scaffolding or the PASS marker: no logged output and no
// unaccounted line, so output from another process cannot be swallowed.
func goVerboseBlock(lines []string, i int) (string, int, bool) {
	if !reGoVerboseRun.MatchString(lines[i]) {
		return "", 0, false
	}
	for j := i + 1; j < len(lines); j++ {
		if !reGoVerboseOK.MatchString(lines[j]) {
			continue
		}
		block := lines[i : j+1]
		seg, reason := scanGoVerbose(block, false)
		if reason != "" || len(seg.kept) > 0 || seg.folded > 0 {
			return "", 0, false
		}
		out, ok, _ := parseGoVerbose(blockText(block))
		return string(out), j + 1, ok
	}
	return "", 0, false
}

// pytestBlock bounds a pytest session: the "test session starts" banner to the
// closing "=== N passed in Xs ===" banner. compressPytest refuses the block
// when any line between is not a clean pytest line.
func pytestBlock(lines []string, i int) (string, int, bool) {
	closing := func(l string) bool { return strings.HasPrefix(l, "=") && rePytestSummary.MatchString(l) }
	return bannerBlock(lines, i, rePytestSessionLine.MatchString, closing, compressPytest)
}

// vitestBlock bounds a vitest run: the "RUN vX.Y.Z" header to the "Duration"
// line. A run that printed anything outside vitest's own grammar (console
// output, notices) stays raw, since that text may belong to another process.
func vitestBlock(lines []string, i int) (string, int, bool) {
	summary, next, ok := bannerBlock(lines, i, reVitestHeaderLine.MatchString, reVitestDurationLine.MatchString, compressVitest)
	if !ok {
		return "", 0, false
	}
	for _, l := range lines[i:next] {
		if !vitestGrammarLine(l) {
			return "", 0, false
		}
	}
	return summary, next, true
}

// vitestGrammarLine reports whether l is a line of vitest's clean grammar.
func vitestGrammarLine(l string) bool {
	return strings.TrimSpace(l) == "" || reVitestHeaderLine.MatchString(l) ||
		reVitestFileLine.MatchString(l) || reVitestStartLine.MatchString(l) ||
		reVitestSummaryLine.MatchString(l) || reVitestDurationLine.MatchString(l)
}

// bannerBlock cuts lines[i:j+1] where lines[i] opens a block and j is the first
// later line that closes it, with no second opening line between. The block is
// replaced by the runner's summary only when compress applied and dropped no
// line.
func bannerBlock(lines []string, i int, open, closes func(string) bool, compress func([]byte) ([]byte, bool, string)) (string, int, bool) {
	if !open(lines[i]) {
		return "", 0, false
	}
	for j := i + 1; j < len(lines); j++ {
		if open(lines[j]) {
			return "", 0, false
		}
		if !closes(lines[j]) {
			continue
		}
		out, ok, _ := compress(blockText(lines[i : j+1]))
		if !ok || droppedLines(out) {
			return "", 0, false
		}
		return string(out), j + 1, true
	}
	return "", 0, false
}

// droppedLines reports whether a summary ends with a "lines dropped" marker.
func droppedLines(out []byte) bool {
	rows := strings.Split(strings.TrimSuffix(string(out), "\n"), "\n")
	return reDroppedMarker.MatchString(rows[len(rows)-1])
}
