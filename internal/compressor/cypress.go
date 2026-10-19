// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var (
	reCypressStat  = regexp.MustCompile(`^\s*(Tests|Passing|Failing|Pending|Skipped):\s+(\d+)\s*$`)
	reCypressError = regexp.MustCompile(`(?i)(?:AssertionError|Error:|uncaught (?:exception|error)|failed|timed out|warning)`)
)

type cypressSummary struct {
	tests    int
	passing  int
	failing  int
	pending  int
	skipped  int
	duration string
	spec     string
}

func isCypress(input []byte) bool {
	s := string(input)
	return strings.Contains(s, "(Run Starting)") || strings.Contains(s, "(Run Finished)")
}

// compressCypress summarizes only a completed, unambiguous, passing Cypress
// CLI results block. Failed or incomplete runs stay byte-identical so their
// spec names and error output remain available for debugging.
func compressCypress(input []byte) ([]byte, bool, string) {
	if reCypressError.Match(input) {
		return input, false, "cypress_error_output_preserved"
	}
	summary, ok := parseCypressSummary(string(input))
	if !ok {
		return input, false, "cypress_incomplete_or_ambiguous_preserved"
	}
	if summary.failing != 0 {
		return input, false, "cypress_failures_preserved"
	}
	if summary.tests != summary.passing+summary.pending+summary.skipped {
		return input, false, "cypress_inconsistent_counts_preserved"
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Cypress: PASS (%d tests: %d passing, %d pending, %d skipped", summary.tests, summary.passing, summary.pending, summary.skipped)
	if summary.spec != "" {
		fmt.Fprintf(&b, "; spec %s", summary.spec)
	}
	if summary.duration != "" {
		fmt.Fprintf(&b, "; %s", summary.duration)
	}
	b.WriteString(")\n")

	out := b.String()
	if len(out) >= len(input) {
		return input, false, "no_compression_gain"
	}
	return []byte(out), true, "compressed Cypress passing summary"
}

func parseCypressSummary(output string) (cypressSummary, bool) {
	var summary cypressSummary
	if !strings.Contains(output, "(Run Finished)") {
		return summary, false
	}

	seen := make(map[string]bool, 5)
	for _, line := range strings.Split(output, "\n") {
		trimmed := strings.TrimSpace(strings.Trim(strings.TrimSpace(line), "│"))
		if matches := reCypressStat.FindStringSubmatch(trimmed); matches != nil {
			key := strings.ToLower(matches[1])
			if seen[key] {
				return cypressSummary{}, false
			}
			seen[key] = true
			value, err := strconv.Atoi(matches[2])
			if err != nil {
				return cypressSummary{}, false
			}
			switch key {
			case "tests":
				summary.tests = value
			case "passing":
				summary.passing = value
			case "failing":
				summary.failing = value
			case "pending":
				summary.pending = value
			case "skipped":
				summary.skipped = value
			}
		}
		if strings.HasPrefix(trimmed, "Duration:") {
			summary.duration = strings.TrimSpace(strings.TrimPrefix(trimmed, "Duration:"))
		}
		if strings.HasPrefix(trimmed, "Spec Ran:") {
			if summary.spec != "" {
				return cypressSummary{}, false
			}
			summary.spec = strings.TrimSpace(strings.TrimPrefix(trimmed, "Spec Ran:"))
		}
	}

	if !seen["tests"] || !seen["passing"] || !seen["failing"] {
		return cypressSummary{}, false
	}
	return summary, true
}
