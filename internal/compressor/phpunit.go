// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	rePHPUnitBanner   = regexp.MustCompile(`^PHPUnit\s+\d+(?:\.\d+){1,2}\s+by\s+.+$`)
	rePHPUnitRuntime  = regexp.MustCompile(`^Runtime:\s+.+$`)
	rePHPUnitConfig   = regexp.MustCompile(`^Configuration:\s+.+$`)
	rePHPUnitProgress = regexp.MustCompile(`^(\.+)\s+(\d+)\s*/\s*(\d+)\s+\(100%\)$`)
	rePHPUnitTime     = regexp.MustCompile(`^Time:\s+([^,]+)(?:,\s*Memory:\s+(.+))?$`)
	rePHPUnitSummary  = regexp.MustCompile(`^OK \((\d+) tests?, (\d+) assertions?\)$`)
)

func isPHPUnit(input []byte) bool {
	return rePHPUnitBanner.MatchString(strings.SplitN(string(input), "\n", 2)[0])
}

var rePHPUnitFailure = regexp.MustCompile(`(?m)^(?:FAILURES!|ERRORS!|INCOMPLETE|RISKY|There were errors)`)

// compressPHPUnit compacts only PHPUnit's complete, default dot-progress
// success transcript. Test output, diagnostics, alternate printers, failed or
// partial summaries, and unknown lines are returned unchanged.
func compressPHPUnit(input []byte) ([]byte, bool, string) {
	if len(input) == 0 || input[len(input)-1] != '\n' || strings.ContainsRune(string(input), '\r') {
		return input, false, "phpunit_incomplete_bytes_preserved"
	}
	lines := strings.Split(string(input), "\n")
	var (
		seenBanner, seenRuntime, seenConfig, seenProgress, seenTime, seenSummary bool
		passed, total, testCount, assertionCount                                 int
		duration                                                                 string
	)

	for _, line := range lines {
		if line == "" {
			continue
		}
		switch {
		case rePHPUnitBanner.MatchString(line):
			if seenBanner || seenProgress || seenSummary {
				return input, false, "phpunit_ambiguous_banner_preserved"
			}
			seenBanner = true
		case rePHPUnitRuntime.MatchString(line):
			if !seenBanner || seenRuntime || seenProgress || seenSummary {
				return input, false, "phpunit_unexpected_runtime_line_preserved"
			}
			seenRuntime = true
		case rePHPUnitConfig.MatchString(line):
			if !seenBanner || seenConfig || seenProgress || seenSummary {
				return input, false, "phpunit_unexpected_configuration_line_preserved"
			}
			seenConfig = true
		case rePHPUnitProgress.MatchString(line):
			if !seenBanner || seenProgress || seenSummary {
				return input, false, "phpunit_ambiguous_progress_preserved"
			}
			m := rePHPUnitProgress.FindStringSubmatch(line)
			passed = len(m[1])
			var err error
			total, err = parsePHPUnitCount(m[3])
			if err != nil {
				return input, false, "phpunit_invalid_progress_count_preserved"
			}
			progressCount, err := parsePHPUnitCount(m[2])
			if err != nil || progressCount != total {
				return input, false, "phpunit_inconsistent_progress_preserved"
			}
			seenProgress = true
		case rePHPUnitTime.MatchString(line):
			if !seenProgress || seenTime || seenSummary {
				return input, false, "phpunit_unexpected_time_line_preserved"
			}
			m := rePHPUnitTime.FindStringSubmatch(line)
			duration = strings.TrimSpace(m[1])
			if m[2] != "" {
				duration += ", " + strings.TrimSpace(m[2])
			}
			seenTime = true
		case rePHPUnitSummary.MatchString(line):
			if !seenProgress || seenSummary {
				return input, false, "phpunit_unexpected_summary_preserved"
			}
			m := rePHPUnitSummary.FindStringSubmatch(line)
			var err error
			testCount, err = parsePHPUnitCount(m[1])
			if err != nil {
				return input, false, "phpunit_invalid_summary_preserved"
			}
			assertionCount, err = parsePHPUnitCount(m[2])
			if err != nil {
				return input, false, "phpunit_invalid_summary_preserved"
			}
			seenSummary = true
		default:
			return input, false, "phpunit_unrecognized_output_preserved"
		}
	}

	if !seenBanner || !seenProgress || !seenSummary || passed != total || testCount != total {
		return input, false, "phpunit_incomplete_or_inconsistent_success_preserved"
	}

	var out strings.Builder
	fmt.Fprintf(&out, "PHPUnit: PASS (%d tests, %d assertions", testCount, assertionCount)
	if duration != "" {
		fmt.Fprintf(&out, " in %s", duration)
	}
	out.WriteString(")\n")
	if out.Len() >= len(input) {
		return input, false, "no_compression_gain"
	}
	return []byte(out.String()), true, fmt.Sprintf("compressed PHPUnit passing summary (%d tests)", testCount)
}

func parsePHPUnitCount(value string) (int, error) {
	var count int
	_, err := fmt.Sscanf(value, "%d", &count)
	return count, err
}
