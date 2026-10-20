// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import (
	"bytes"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// reFlutterProgress is one status segment of the Dart test reporter that
// `flutter test` and `dart test` share: `00:03 +12 ~1: name`. The failure
// counter (`-N`) is parsed only so it can be refused.
var reFlutterProgress = regexp.MustCompile(`^(\d{2,}:\d{2}) \+(\d+)(?: -(\d+))?(?: ~(\d+))?: (.+)$`)

// reFlutterTestFailure marks a failing, erroring or diagnostic run. Detection
// reports it as an error so the raw output is kept.
var reFlutterTestFailure = regexp.MustCompile(`\[E\]|Some tests failed|\+\d+ -\d+|Expected:|Actual:|EXCEPTION CAUGHT`)

// reFlutterDanger marks segment text that must never be dropped: a print can
// imitate the status format, so such a segment makes the run ambiguous.
var reFlutterDanger = regexp.MustCompile(`(?i)error|exception|fail|warning|panic|fatal|secret|token|password`)

const flutterAllPassed = "All tests passed!"

// flutterSegments splits input into reporter segments. The compact reporter
// rewrites one line with carriage returns even without a TTY; the expanded
// one prints a line per update. Both are a sequence of newline-terminated
// lines, each holding one or more segments separated by "\r".
func flutterSegments(line string) []string {
	segs := strings.Split(line, "\r")
	if len(segs) > 1 && segs[0] == "" {
		segs = segs[1:]
	}
	return segs
}

// isFlutterTest recognises the reporter by its first segment only.
func isFlutterTest(input []byte) bool {
	end := bytes.IndexAny(input, "\r\n")
	if end == 0 && input[0] == '\r' {
		rest := input[1:]
		end = bytes.IndexAny(rest, "\r\n")
		if end < 0 {
			end = len(rest)
		}
		return reFlutterProgress.MatchString(strings.TrimRight(string(rest[:end]), " "))
	}
	if end < 0 {
		end = len(input)
	}
	return reFlutterProgress.MatchString(strings.TrimRight(string(input[:end]), " "))
}

// compressFlutterTest compacts only a clean `flutter test` / `dart test`
// transcript: every segment is a progress update with no failure counter,
// counters never go backwards, and the last segment is the all-passed line.
// Test print output, warnings, tool chatter, ANSI, CRLF and anything else is
// returned unchanged.
func compressFlutterTest(input []byte) ([]byte, bool, string) {
	if len(input) == 0 || input[len(input)-1] != '\n' {
		return input, false, "flutter_test_incomplete_bytes_preserved"
	}
	if bytes.Contains(input, []byte("\r\n")) || bytes.IndexByte(input, 0x1b) != -1 || reFlutterTestFailure.Match(input) {
		return input, false, "flutter_test_ambiguous_bytes_preserved"
	}
	lines := strings.Split(string(input[:len(input)-1]), "\n")
	var (
		passed, skipped, count int
		clock, last            string
	)
	for _, line := range lines {
		if line == "" {
			return input, false, "flutter_test_unexpected_blank_line_preserved"
		}
		for _, seg := range flutterSegments(line) {
			seg = strings.TrimRight(seg, " ")
			m := reFlutterProgress.FindStringSubmatch(seg)
			// A double space or tab inside a segment means other text is
			// glued to the status line (test print output).
			if m == nil || strings.Contains(seg, "  ") || strings.ContainsRune(seg, '\t') {
				return input, false, "flutter_test_unrecognized_output_preserved"
			}
			if reFlutterDanger.MatchString(m[5]) {
				return input, false, "flutter_test_danger_word_preserved"
			}
			if m[3] != "" && m[3] != "0" {
				return input, false, "flutter_test_failure_counter_preserved"
			}
			p, _ := strconv.Atoi(m[2])
			s := 0
			if m[4] != "" {
				s, _ = strconv.Atoi(m[4])
			}
			if p < passed || s < skipped {
				return input, false, "flutter_test_counter_regression_preserved"
			}
			if last == flutterAllPassed {
				return input, false, "flutter_test_output_after_summary_preserved"
			}
			passed, skipped, clock, last = p, s, m[1], m[5]
			count++
		}
	}
	if last != flutterAllPassed || passed == 0 || count < 2 {
		return input, false, "flutter_test_incomplete_or_inconsistent_success_preserved"
	}
	var out strings.Builder
	fmt.Fprintf(&out, "dart/flutter test: PASS (%d passed", passed)
	if skipped > 0 {
		fmt.Fprintf(&out, ", %d skipped", skipped)
	}
	fmt.Fprintf(&out, " in %s)\n", clock)
	if out.Len() >= len(input) {
		return input, false, "no_compression_gain"
	}
	return []byte(out.String()), true, fmt.Sprintf("compressed dart/flutter test passing summary (%d tests)", passed)
}
