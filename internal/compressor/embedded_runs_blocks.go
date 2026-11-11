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

// blockScan holds the lines of a transcript and, per runner, the index of the
// next opening and closing line at or after each line (len(lines) when none).
// One backward pass builds them, so finding a block's end costs O(1) instead
// of a rescan from every opener.
type blockScan struct {
	lines                      []string
	goOK                       []int
	pyOpen, pyClose            []int
	vitestOpen, vitestDuration []int
}

func newBlockScan(lines []string) *blockScan {
	n := len(lines)
	s := &blockScan{lines: lines}
	next := func(match func(string) bool) []int {
		idx := make([]int, n+1)
		idx[n] = n
		for i := n - 1; i >= 0; i-- {
			idx[i] = idx[i+1]
			if match(lines[i]) {
				idx[i] = i
			}
		}
		return idx
	}
	s.goOK = next(reGoVerboseOK.MatchString)
	s.pyOpen = next(rePytestSessionLine.MatchString)
	s.pyClose = next(pytestClosingLine)
	s.vitestOpen = next(reVitestHeaderLine.MatchString)
	s.vitestDuration = next(reVitestDurationLine.MatchString)
	return s
}

func pytestClosingLine(l string) bool {
	return strings.HasPrefix(l, "=") && rePytestSummary.MatchString(l)
}

func blockText(lines []string) []byte { return []byte(strings.Join(lines, "\n") + "\n") }

// goVerboseBlock bounds a `go test -v` package: from its first "=== RUN" line
// to the package's "ok" line. The block folds only when every line is a test
// run, outcome, scaffolding or the PASS marker: no logged output and no
// unaccounted line, so output from another process cannot be swallowed. With
// no "ok" line after it, line i alone is passed over, so a runner block later
// in the transcript can still fold.
func goVerboseBlock(s *blockScan, i int) (string, int, bool) {
	if !reGoVerboseRun.MatchString(s.lines[i]) {
		return "", i + 1, false
	}
	j := s.goOK[i+1]
	if j == len(s.lines) {
		return "", i + 1, false
	}
	block := s.lines[i : j+1]
	seg, reason := scanGoVerbose(block, false)
	if reason != "" || len(seg.kept) > 0 || seg.folded > 0 {
		return "", j + 1, false
	}
	out, ok, _ := parseGoVerbose(blockText(block))
	return string(out), j + 1, ok
}

// pytestBlock bounds a pytest session: the "test session starts" banner to the
// closing "=== N passed in Xs ===" banner. compressPytest refuses the block
// when any line between is not a clean pytest line.
func pytestBlock(s *blockScan, i int) (string, int, bool) {
	return bannerBlock(s, i, s.pyOpen, s.pyClose, compressPytest)
}

// vitestBlock bounds a vitest run: the "RUN vX.Y.Z" header to the "Duration"
// line. A run that printed anything outside vitest's own grammar (console
// output, notices) stays raw, since that text may belong to another process.
func vitestBlock(s *blockScan, i int) (string, int, bool) {
	summary, next, ok := bannerBlock(s, i, s.vitestOpen, s.vitestDuration, compressVitest)
	if !ok {
		return "", next, false
	}
	for _, l := range s.lines[i:next] {
		if !vitestGrammarLine(l) {
			return "", next, false
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

// bannerBlock cuts lines[i:j+1] where line i opens a block (open[i] == i) and
// j is the first later closing line, with no second opening line between. The
// block is replaced by the runner's summary only when compress applied and
// dropped no line. A second opener first: the scan resumes there. No closer:
// line i alone is passed over.
func bannerBlock(s *blockScan, i int, open, closes []int, compress func([]byte) ([]byte, bool, string)) (string, int, bool) {
	if open[i] != i {
		return "", i + 1, false
	}
	j := closes[i+1]
	if j == len(s.lines) {
		return "", i + 1, false
	}
	if o := open[i+1]; o < j {
		return "", o, false
	}
	out, ok, _ := compress(blockText(s.lines[i : j+1]))
	if !ok || droppedLines(out) {
		return "", j + 1, false
	}
	return string(out), j + 1, true
}

// droppedLines reports whether a summary ends with a "lines dropped" marker.
func droppedLines(out []byte) bool {
	rows := strings.Split(strings.TrimSuffix(string(out), "\n"), "\n")
	return reDroppedMarker.MatchString(rows[len(rows)-1])
}
