// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/tiagovilasboas/downshift-ai/internal/lazyre"
)

var (
	reCargoTestSummary = lazyre.New(`(?m)^\s*test result: ok\.\s+(\d+)\s+passed;\s+0\s+failed;.*?finished in\s+([^\n\r]+?)\s*$`)
	reCargoTestHeader  = lazyre.New(`(?m)^running\s+\d+\s+tests`)
	reCargoTestError   = lazyre.New(`(?m)(?:FAILED|error\[E[^\]]*\]:|warning:|panicked at)`)
)

func isCargoTest(input []byte) bool {
	return reCargoTestSummary.Match(input) ||
		reCargoTestHeader.Match(input) ||
		bytes.Contains(input, []byte("test result:")) ||
		reCargoTestError.Match(input)
}

// cargoTestLinePatterns are the only line shapes a clean cargo test run emits.
// Recognition is the rule: a line that matches none of them (a compiler
// diagnostic, a panic, a failure marker, an injected warning) keeps the whole
// transcript raw, whatever its words are.
var cargoTestLinePatterns = []*lazyre.Regexp{
	lazyre.New(`^\s*Compiling \S+ v\S+ \(.*\)$`),
	lazyre.New(`^\s*Finished .+ profile \[[^\]]+\] target\(s\) in \S+$`),
	lazyre.New(`^\s*Running (?:unittests )?\S+ \(.*\)$`),
	lazyre.New(`^\s*Doc-tests \S+$`),
	lazyre.New(`^running \d+ tests?$`),
	lazyre.New(`^test \S.* \.\.\. ok$`),
	lazyre.New(`^test result: ok\. \d+ passed; 0 failed; \d+ ignored; \d+ measured; \d+ filtered out; finished in \S+$`),
}

// cargoTestLinesRecognised reports whether every non-empty line of input is a
// plain cargo test success line. Empty lines are structural and always allowed.
func cargoTestLinesRecognised(input []byte) bool {
	for _, line := range bytes.Split(input, []byte("\n")) {
		if len(line) == 0 {
			continue
		}
		// Free text carrying an error word is never a plain success line, even
		// when its shape fits a pattern (e.g. "Running unittests x (error: boom)").
		if bytes.Contains(bytes.ToLower(line), []byte("error")) || !matchesAnyCargoTestLine(line) {
			return false
		}
	}
	return true
}

func matchesAnyCargoTestLine(line []byte) bool {
	for _, re := range cargoTestLinePatterns {
		if re.Match(line) {
			return true
		}
	}
	return false
}

// compressCargoTest compacts clean cargo test output to a one-line summary:
// Cargo test: PASS (N passed in <time>)
// If not strictly shorter than raw input, the input is returned.
func compressCargoTest(input []byte) ([]byte, bool, string) {
	if !cargoTestLinesRecognised(input) {
		return input, false, "cargo_test_unrecognised_line_preserved"
	}
	matches := reCargoTestSummary.FindAllSubmatch(input, -1)
	if len(matches) == 0 {
		return input, false, "cargo_test_missing_summary"
	}
	var totalPassed int
	var timeStr string
	for _, m := range matches {
		var p int
		fmt.Sscanf(string(m[1]), "%d", &p)
		totalPassed += p
		if timeStr == "" || p > 0 {
			timeStr = strings.TrimSpace(string(m[2]))
		}
	}

	out := fmt.Sprintf("Cargo test: PASS (%d passed in %s)\n", totalPassed, timeStr)
	if len(out) >= len(input) {
		return input, false, "no_compression_gain"
	}
	return []byte(out), true, fmt.Sprintf("compressed cargo test summary (%d passed in %s)", totalPassed, timeStr)
}
