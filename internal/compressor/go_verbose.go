// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	reGoVerboseRun     = regexp.MustCompile(`(?m)^=== RUN\s+([A-Za-z0-9_]+)$`)
	reGoVerboseOutcome = regexp.MustCompile(`^--- (PASS|FAIL|SKIP): ([A-Za-z0-9_]+) \([^()]*\)$`)
	reGoVerboseOK      = regexp.MustCompile(`^ok\s+(\S+)\s+(\S+)(?:\s+coverage: .+)?$`)
	reGoVerboseFAIL    = regexp.MustCompile(`^FAIL\s+(\S+)\s+(\S+)$`)
)

// parseGoVerbose compacts a complete, single-package `go test -v` transcript
// only when every test passes and emits no output. Failures, test logs, and
// unrecognized, nested, incomplete, or ambiguous output are returned unchanged.
func parseGoVerbose(input []byte) ([]byte, bool, string) {
	lines := strings.Split(string(input), "\n")
	var blocks [][]string
	var current []string
	var currentName string
	var passed, failed, skipped int
	var packageName, packageTime string
	terminal := ""
	seenPassMarker := false

	for _, line := range lines {
		if line == "" {
			continue
		}
		if m := reGoVerboseRun.FindStringSubmatch(line); m != nil {
			if current != nil {
				return input, false, "go_test_verbose_unexpected_nested_or_duplicate_run"
			}
			currentName = m[1]
			current = []string{line}
			continue
		}
		if m := reGoVerboseOutcome.FindStringSubmatch(line); m != nil {
			if current == nil || currentName != m[2] {
				return input, false, "go_test_verbose_unmatched_test_outcome"
			}
			current = append(current, line)
			blocks = append(blocks, current)
			current = nil
			currentName = ""
			switch m[1] {
			case "PASS":
				passed++
			case "FAIL":
				failed++
			case "SKIP":
				skipped++
			}
			continue
		}
		if current != nil {
			// Captured test output is safe to discard only for a passing test.
			// Keep it in the block until the outcome is known.
			current = append(current, line)
			continue
		}
		if line == "PASS" || line == "FAIL" {
			if terminal != "" || line == "PASS" && failed > 0 || line == "FAIL" && failed == 0 {
				return input, false, "go_test_verbose_ambiguous_terminal_marker"
			}
			terminal = line
			seenPassMarker = line == "PASS"
			continue
		}
		if m := reGoVerboseOK.FindStringSubmatch(line); m != nil {
			if terminal != "PASS" || packageName != "" {
				return input, false, "go_test_verbose_unexpected_package_summary"
			}
			packageName, packageTime = m[1], m[2]
			continue
		}
		if m := reGoVerboseFAIL.FindStringSubmatch(line); m != nil {
			if terminal != "FAIL" || packageName != "" {
				return input, false, "go_test_verbose_unexpected_package_summary"
			}
			packageName, packageTime = m[1], m[2]
			continue
		}
		return input, false, "go_test_verbose_unrecognized_line"
	}
	if current != nil || len(blocks) == 0 || packageName == "" || terminal == "" {
		return input, false, "go_test_verbose_incomplete_transcript"
	}
	if terminal == "PASS" && (failed != 0 || !seenPassMarker) || terminal == "FAIL" && failed == 0 {
		return input, false, "go_test_verbose_result_mismatch"
	}
	if terminal == "PASS" && len(blocks) != passed+skipped || terminal == "FAIL" && len(blocks) != passed+failed+skipped {
		return input, false, "go_test_verbose_result_mismatch"
	}
	if terminal != "PASS" || failed > 0 {
		return input, false, "go_test_verbose_failure_preserved"
	}

	var out strings.Builder
	fmt.Fprintf(&out, "Go test -v: PASS (%d passed, 0 failed, %d skipped) in %s\n", passed, skipped, packageTime)
	if packageName != "" {
		fmt.Fprintf(&out, "Package: %s\n", packageName)
	}
	for _, block := range blocks {
		if len(block) > 2 {
			// Test output may carry useful facts even when the test passes.
			return input, false, "go_test_verbose_test_output_preserved"
		}
	}
	compacted := []byte(out.String())
	if len(compacted) >= len(input) {
		return input, false, "no_compression_gain"
	}
	return compacted, true, fmt.Sprintf("compressed go test -v results (%d passed, %d failed, %d skipped)", passed, failed, skipped)
}
