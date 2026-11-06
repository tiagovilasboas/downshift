// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import (
	"fmt"
	"regexp"
	"strings"
)

const (
	// pytestPathChars is the closed character set of a pytest path value.
	pytestPathChars = `[A-Za-z0-9_./@+~-]+`
	// pytestPathValue is a relative or absolute path: the closed set, with at
	// least one separator or dot, so a bare word such as FAILED is refused.
	pytestPathValue = `[A-Za-z0-9_@+~-]*[./][A-Za-z0-9_./@+~-]*`
)

var (
	rePytestSummary     = regexp.MustCompile(`(?m)^=+\s*(?:(\d+)\s+skipped,\s*)?(\d+)\s+passed(?:,\s*(\d+)\s+skipped)?\s+in\s+(\d+(?:\.\d+)?s) =+$`)
	rePytestHeader      = regexp.MustCompile(`(?m)^=+\s*test session starts\s*=+`)
	rePytestFailSummary = regexp.MustCompile(`(?m)^=+\s*.*?(?:failed|error).*?\s+in\s+`)
	// rePytestPlainLine lists every line shape a clean pytest run emits. A
	// transcript is compacted only when each line matches one of these shapes
	// (or is the single passing summary); any other line keeps the output raw.
	// Path values use one closed character set and no spaces; a value outside
	// it keeps the whole transcript raw.
	rePytestPlainLine = regexp.MustCompile(`^(?:` +
		`|=+ test session starts =+` +
		`|platform [A-Za-z0-9_.-]+ -- Python [0-9][0-9A-Za-z.+-]*(?:, (?:pytest|pluggy)-[0-9][0-9A-Za-z.+-]*)*(?: -- /` + pytestPathChars + `)?` +
		`|rootdir: /` + pytestPathChars +
		`|configfile: ` + pytestPathValue +
		`|cachedir: ` + pytestPathValue +
		`|plugins: [A-Za-z0-9_.-]+-[0-9][A-Za-z0-9_.+-]*(?:, [A-Za-z0-9_.-]+-[0-9][A-Za-z0-9_.+-]*)*` +
		`|collected \d+ items?(?: / \d+ deselected)?` +
		`|` + pytestPathChars + `\.py [.s]+(?: +\[ *\d+%\])?` +
		`)$`)
)

func isPytest(input []byte) bool {
	return rePytestSummary.Match(input) ||
		rePytestHeader.Match(input) ||
		rePytestFailSummary.Match(input)
}

// isPlainPytestPass reports whether every line of input is a recognised plain
// pytest line and exactly one line is a passing summary. A failure, a warning,
// a traceback or any other unrecognised line, injected or not, makes it false.
func isPlainPytestPass(input []byte) bool {
	summaries := 0
	for _, line := range strings.Split(strings.TrimSuffix(string(input), "\n"), "\n") {
		switch {
		case rePytestSummary.MatchString(line):
			summaries++
		case !rePytestPlainLine.MatchString(line):
			return false
		}
	}
	return summaries == 1
}

// compressPytest compacts clean pytest output to a one-line summary:
// Pytest: PASS (N passed[ , M skipped] in <time>)
// If not strictly shorter than raw input, the input is returned.
func compressPytest(input []byte) ([]byte, bool, string) {
	if !isPlainPytestPass(input) {
		return input, false, "pytest_unrecognised_line_preserved"
	}
	m := rePytestSummary.FindSubmatch(input)
	if m == nil {
		return input, false, "pytest_missing_summary"
	}
	skippedCount := string(m[1])
	if skippedCount == "" {
		skippedCount = string(m[3])
	}
	passedCount := string(m[2])
	timeStr := strings.TrimSpace(string(m[4]))

	var out string
	if skippedCount != "" {
		out = fmt.Sprintf("Pytest: PASS (%s passed, %s skipped in %s)\n", passedCount, skippedCount, timeStr)
	} else {
		out = fmt.Sprintf("Pytest: PASS (%s passed in %s)\n", passedCount, timeStr)
	}

	if len(out) >= len(input) {
		return input, false, "no_compression_gain"
	}
	var reason string
	if skippedCount != "" {
		reason = fmt.Sprintf("compressed pytest summary (%s passed, %s skipped)", passedCount, skippedCount)
	} else {
		reason = fmt.Sprintf("compressed pytest summary (%s passed)", passedCount)
	}
	return []byte(out), true, reason
}
