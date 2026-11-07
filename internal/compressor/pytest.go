// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import (
	"fmt"
	"regexp"
	"strconv"
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
		`|collecting \.\.\. collected \d+ items?(?: / \d+ deselected)?` +
		`)$`)
	// rePytestProgress is a progress line of a default run: outcome characters
	// of passed (".") and skipped ("s") tests, with the file name on the first
	// line of a file and none on the lines pytest wraps at the terminal width.
	rePytestProgress = regexp.MustCompile(`^(?:` + pytestPathChars + `\.py )?([.s]+)(?: +\[ *\d+%\])?$`)
	// rePytestPassedVerbose is a -v line of a passed test. The node id is
	// printable ASCII (pytest escapes the rest), so a line is dropped only when
	// it is a path, "::", an id and the outcome PASSED.
	rePytestPassedVerbose = regexp.MustCompile(`^` + pytestPathChars + `::[ -~]+ PASSED(?: +\[ *\d+%\])?$`)
	// rePytestTestpathsWord is a bare directory name in a testpaths value.
	rePytestTestpathsWord = regexp.MustCompile(`^[a-z0-9_-]+$`)
	rePytestPathValueOnly = regexp.MustCompile(`^` + pytestPathValue + `$`)
	rePytestDangerWord    = regexp.MustCompile(`(?i)fail|error|warn|panic|assert|except|trace|skip|xfail`)
)

// isPytestTestpathsLine reports whether line is the "testpaths: a b" line of the
// session header. A value is a path (it has a separator or a dot) or a lowercase
// directory name that is not a danger word, so "testpaths: FAILED" stays unknown.
func isPytestTestpathsLine(line string) bool {
	rest, ok := strings.CutPrefix(line, "testpaths: ")
	if !ok || rest == "" {
		return false
	}
	for _, w := range strings.Split(rest, " ") {
		switch {
		case rePytestPathValueOnly.MatchString(w):
		case rePytestTestpathsWord.MatchString(w) && !rePytestDangerWord.MatchString(w):
		default:
			return false
		}
	}
	return true
}

// isPytestPlainLine reports whether line is a shape a clean run emits.
func isPytestPlainLine(line string) bool {
	return rePytestPlainLine.MatchString(line) || isPytestTestpathsLine(line)
}

func isPytest(input []byte) bool {
	return rePytestSummary.Match(input) ||
		rePytestHeader.Match(input) ||
		rePytestFailSummary.Match(input)
}

// isPlainPytestPass reports whether every line of input is a recognised plain
// pytest line, exactly one line is a passing summary, and the progress lines
// account for exactly the tests the summary counts: each "." or PASSED line one
// passed test, each "s" one skipped test. A failure, a warning, a traceback or
// any other unrecognised line, injected or not, makes it false, and so does a
// count that does not add up (a line pytest wrote that this reader dropped).
func isPlainPytestPass(input []byte) bool {
	summaries, passedSeen, skippedSeen := 0, 0, 0
	wantPassed, wantSkipped := -1, -1
	for _, line := range strings.Split(strings.TrimSuffix(string(input), "\n"), "\n") {
		if m := rePytestSummary.FindStringSubmatch(line); m != nil {
			summaries++
			skipped := m[1]
			if skipped == "" {
				skipped = m[3]
			}
			wantPassed, _ = strconv.Atoi(m[2])
			wantSkipped, _ = strconv.Atoi(skipped)
			continue
		}
		if m := rePytestProgress.FindStringSubmatch(line); m != nil {
			passedSeen += strings.Count(m[1], ".")
			skippedSeen += strings.Count(m[1], "s")
			continue
		}
		if rePytestPassedVerbose.MatchString(line) {
			passedSeen++
			continue
		}
		if !isPytestPlainLine(line) {
			return false
		}
	}
	return summaries == 1 && passedSeen == wantPassed && skippedSeen == wantSkipped
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
