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
	// Subtest scaffolding: a nested run header, and its indented outcome line.
	reGoVerboseSubRun     = regexp.MustCompile(`^=== RUN\s+(\S+)$`)
	reGoVerbosePauseCont  = regexp.MustCompile(`^=== (?:PAUSE|CONT|NAME)\s+\S+$`)
	reGoVerboseSubOutcome = regexp.MustCompile(`^\s+--- (PASS|FAIL|SKIP): (\S+) \([^()]*\)$`)
)

// parseGoVerbose compacts a complete, single-package `go test -v` transcript
// only when every test passes and emits no output. Subtests count as
// scaffolding of their parent only when they are plain PASS or SKIP run/outcome
// lines under it. Failures, test logs, parallel markers (=== PAUSE, === CONT),
// and unrecognized, incomplete, or ambiguous output are returned unchanged.
func parseGoVerbose(input []byte) ([]byte, bool, string) {
	lines := strings.Split(string(input), "\n")
	var blocks [][]string
	var current []string
	var currentName string
	var passed, failed, skipped, subtests int
	var kept []string
	folded := 0
	var packageName, packageTime, lastName string
	terminal := ""
	seenPassMarker := false

	for _, line := range lines {
		if line == "" {
			continue
		}
		if current != nil {
			if m := reGoVerboseSubRun.FindStringSubmatch(line); m != nil && strings.HasPrefix(m[1], currentName+"/") {
				current = append(current, line)
				continue
			}
		} else if m := reGoVerboseSubOutcome.FindStringSubmatch(line); m != nil && lastName != "" && strings.HasPrefix(m[2], lastName+"/") {
			// Go prints a parent's own outcome first and its indented subtest
			// outcomes right after it.
			if m[1] == "FAIL" {
				return input, false, "go_test_verbose_failure_preserved"
			}
			subtests++
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
			lastName = currentName
			currentName = ""
			switch m[1] {
			case "PASS":
				passed++
				if logged, foldable := classifyPassedBlock(current0(blocks)); !foldable {
					kept = append(kept, current0(blocks)...)
				} else if logged {
					folded++
				}
			case "FAIL":
				failed++
			case "SKIP":
				skipped++
				if len(current0(blocks)) > 2 {
					kept = append(kept, current0(blocks)...)
				}
			}
			continue
		}
		if current != nil {
			// Captured test output is only folded once the outcome is known.
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
		// A line outside every test block that we cannot account for stays
		// verbatim, unless it carries a failure signal: then the run is raw.
		if goFoldDanger.MatchString(line) {
			return input, false, "go_test_verbose_danger_line_preserved"
		}
		kept = append(kept, line)
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
	if subtests > 0 {
		fmt.Fprintf(&out, "Subtests: %d passed or skipped under those tests\n", subtests)
	}
	if packageName != "" {
		fmt.Fprintf(&out, "Package: %s\n", packageName)
	}
	if folded > 0 {
		fmt.Fprintf(&out, "%d passed tests logged output (folded)\n", folded)
	}
	if len(kept) > 0 {
		out.WriteString("Kept verbatim:\n")
		for _, l := range kept {
			out.WriteString(l + "\n")
		}
	}
	compacted := []byte(out.String())
	if len(compacted) >= len(input) {
		return input, false, "no_compression_gain"
	}
	return compacted, true, fmt.Sprintf("compressed go test -v results (%d passed, %d failed, %d skipped)", passed, failed, skipped)
}

// current0 returns the block that was just closed.
func current0(blocks [][]string) []string { return blocks[len(blocks)-1] }

// classifyPassedBlock decides whether a passed test's block can fold into the
// counts. Scaffolding (subtest run headers, PAUSE/CONT markers) folds silently;
// indented log lines fold and report logged=true. Any unindented unknown line,
// or a line with a failure signal, makes the block non-foldable (kept verbatim).
func classifyPassedBlock(block []string) (logged, foldable bool) {
	for _, l := range block[1 : len(block)-1] {
		switch {
		case goFoldDanger.MatchString(l):
			return false, false
		case reGoVerboseSubRun.MatchString(l), reGoVerbosePauseCont.MatchString(l):
		case strings.HasPrefix(l, " ") || strings.HasPrefix(l, "\t"):
			logged = true
		default:
			return false, false
		}
	}
	return logged, true
}
