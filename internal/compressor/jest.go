// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	reJestSuites    = regexp.MustCompile(`(?m)^Test Suites:\s+(?:(\d+) skipped,\s*)?(\d+) passed,\s+(\d+) total\s*$`)
	reJestTests     = regexp.MustCompile(`(?m)^Tests:\s+(?:(\d+) skipped,\s*)?(?:(\d+) todo,\s*)?(\d+) passed,\s+(\d+) total\s*$`)
	reJestHeader    = regexp.MustCompile(`(?m)^\s*(?:PASS|Test Suites:)\s+`)
	reJestFail      = regexp.MustCompile(`(?m)^\s*(?:FAIL|Test Suites:.*\bfailed\b|Tests:.*\bfailed\b|Snapshots:.*\bfailed\b)`)
	reJestError     = regexp.MustCompile(`(?m)(?:^\s*Error:|^\s*●|\bSnapshot\s+Summary\b|\bTest suite failed to run\b)`)
	reJestPass      = regexp.MustCompile(`^PASS\s+\S.*$`)
	reJestSnapshots = regexp.MustCompile(`^Snapshots:\s+(?:(?:\d+ (?:failed|passed|obsolete),\s*)*\d+ total|\d+ total)$`)
	reJestTime      = regexp.MustCompile(`^Time:\s+\d+(?:\.\d+)? s(?:, estimated \d+(?:\.\d+)? s)?$`)
)

func isJest(input []byte) bool {
	return reJestHeader.Match(input) || reJestSuites.Match(input) || reJestTests.Match(input) || reJestFail.Match(input)
}

// compressJest compacts a clean Jest summary while retaining skipped and todo
// test counts. It requires both suite and test summaries to avoid treating
// incomplete or otherwise unrecognized output as a successful run.
func compressJest(input []byte) ([]byte, bool, string) {
	body := dropScriptBanner(input)
	kept, dropped := foldJestConsole(body)
	suites := reJestSuites.FindSubmatch(body)
	tests := reJestTests.FindSubmatch(body)
	if suites == nil || tests == nil {
		return input, false, "jest_missing_summary"
	}
	if len(suites) < 4 || len(tests) < 5 {
		return input, false, "jest_invalid_summary"
	}
	var details []string
	if tests[1] != nil {
		details = append(details, string(tests[1])+" skipped")
	}
	if tests[2] != nil {
		details = append(details, string(tests[2])+" todo")
	}
	details = append(details, string(tests[3])+" passed across "+string(suites[3])+" suites")
	if suites[1] != nil {
		unit := "suites"
		if string(suites[1]) == "1" {
			unit = "suite"
		}
		details = append(details, string(suites[1])+" "+unit+" skipped")
	}
	out := fmt.Sprintf("Jest: PASS (%s)\n", strings.Join(details, ", "))
	for _, l := range kept {
		out += l + "\n"
	}
	if dropped > 0 {
		out += fmt.Sprintf("[%d console output lines dropped]\n", dropped)
	}
	if len(out) >= len(input) {
		return input, false, "no_compression_gain"
	}
	return []byte(out), true, fmt.Sprintf("compressed jest summary (%s tests across %s suites)", tests[3], suites[3])
}

// foldJestConsole splits a clean jest transcript into the lines to keep and a
// count of the console output lines it drops. The grammar lines (PASS files,
// the summaries) are neither; any other line is console output: up to 20 stay
// verbatim, above that only danger words, "warn" and coverage table rows stay.
func foldJestConsole(input []byte) (kept []string, dropped int) {
	var console []consoleLine
	for _, line := range strings.Split(strings.TrimSuffix(string(input), "\n"), "\n") {
		switch {
		case strings.TrimSpace(line) == "", reJestPass.MatchString(line),
			reJestSuites.MatchString(line), reJestTests.MatchString(line),
			reJestSnapshots.MatchString(line), reJestTime.MatchString(line),
			line == "Ran all test suites.":
		default:
			keep := reKeepLine.MatchString(line) || (strings.Contains(line, "|") && strings.Contains(line, "%"))
			console = append(console, consoleLine{line, keep})
		}
	}
	return foldConsole(console)
}
