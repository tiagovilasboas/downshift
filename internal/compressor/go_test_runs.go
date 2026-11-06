// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"
)

// FormatGoTestRuns is the output of another program (make, a git hook run by
// git commit or git push) that carries runs of clean go test package lines.
const FormatGoTestRuns Format = "go_test_runs"

// minGoTestRun is the shortest run of package lines worth folding.
const minGoTestRun = 3

// reGoTestRunLine is one clean go test package line: "ok  <pkg>  0.31s" or
// "(cached)", with an optional coverage figure, or "?  <pkg>  [no test files]".
var reGoTestRunLine = regexp.MustCompile(`^(?:ok\s+\S+\s+(?:\d+(?:\.\d+)?s|\(cached\))(?:\s+coverage:\s+.+)?` +
	`|\?\s+\S+\s+\[no test files\]\s*)$`)

// reGoTestFailure marks go test failure output: a failed test or subtest, a
// failed package, a panic or a race report. Anchored so that words like
// "failed_count" do not match.
var reGoTestFailure = regexp.MustCompile(`(?m)^\s*--- FAIL|^FAIL(?:\s|$)|^panic:|WARNING: DATA RACE`)

// FoldGoTestRuns compacts the clean go test runs inside another program's
// output. Each run of minGoTestRun or more consecutive package lines folds
// into the go test summary; every other line is kept as it is, so nothing the
// program said is dropped. It reads the same text CompressExit reads
// (screenText) and returns the raw bytes when no run folds, the result is
// not shorter, or the output carries a go test failure, so a "Go test: PASS"
// summary never sits next to one. Callers use it for a zero exit only.
func FoldGoTestRuns(input []byte) Result {
	res := Result{Output: input, Format: FormatGoTestRuns, OriginalBytes: len(input), ReducedBytes: len(input)}
	switch {
	case len(input) == 0:
		res.Reason = "empty_input"
		return res
	case len(input) > MaxProcessableBytes:
		res.Reason = "input_exceeds_memory_bound"
		return res
	case bytes.IndexByte(input, 0) != -1 || !utf8.Valid(input):
		res.Reason = "binary_or_invalid_utf8"
		return res
	}
	text, reason := screenText(input)
	if reason != "" {
		res.Reason = reason
		return res
	}
	if reGoTestFailure.Match(text) {
		res.Reason = "go_test_runs_failure_preserved"
		return res
	}
	lines := strings.Split(strings.TrimSuffix(string(text), "\n"), "\n")
	var out strings.Builder
	folded := 0
	for i := 0; i < len(lines); {
		j := i
		for j < len(lines) && reGoTestRunLine.MatchString(lines[j]) {
			j++
		}
		if j-i >= minGoTestRun {
			if sum, ok, _ := foldGoTestPlain([]byte(strings.Join(lines[i:j], "\n") + "\n")); ok {
				out.Write(sum)
				folded += j - i
				i = j
				continue
			}
		}
		if j == i {
			j++
		}
		for ; i < j; i++ {
			out.WriteString(lines[i])
			out.WriteByte('\n')
		}
	}
	if folded == 0 {
		res.Reason = "go_test_runs_none_folded"
		return res
	}
	if out.Len() >= len(input) {
		res.Reason = "no_compression_gain"
		return res
	}
	res.Applied = true
	res.Output = []byte(out.String())
	res.ReducedBytes = out.Len()
	res.Reason = fmt.Sprintf("folded %d go test package lines", folded)
	return res
}
