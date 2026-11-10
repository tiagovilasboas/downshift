// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import (
	"fmt"
	"strings"

	"github.com/tiagovilasboas/downshift-ai/internal/lazyre"
)

var (
	reGoVerboseRun     = lazyre.New(`(?m)^=== RUN\s+([A-Za-z0-9_]+)$`)
	reGoVerboseOutcome = lazyre.New(`^--- (PASS|FAIL|SKIP): ([A-Za-z0-9_]+) \([^()]*\)$`)
	reGoVerboseOK      = lazyre.New(`^ok\s+(\S+)\s+(\S+)(?:\s+coverage: .+)?$`)
	reGoVerboseFAIL    = lazyre.New(`^FAIL\s+(\S+)\s+(\S+)$`)
	// Subtest scaffolding: a nested run header, and its indented outcome line.
	reGoVerboseSubRun     = lazyre.New(`^=== RUN\s+(\S+)$`)
	reGoVerbosePauseCont  = lazyre.New(`^=== (?:PAUSE|CONT|NAME)\s+(\S+)$`)
	reGoVerboseSubOutcome = lazyre.New(`^\s+--- (PASS|FAIL|SKIP): (\S+) \([^()]*\)$`)
)

// parseGoVerbose compacts a complete, single-package `go test -v` transcript
// only when every test passes and emits no output. Subtests count as
// scaffolding of their parent only when they are plain PASS or SKIP run/outcome
// lines under it. Failures, test logs, parallel markers (=== PAUSE, === CONT),
// and unrecognized, incomplete, or ambiguous output are returned unchanged.
func parseGoVerbose(input []byte) ([]byte, bool, string) {
	seg, reason := scanGoVerbose(strings.Split(string(input), "\n"), false)
	if reason != "" {
		return input, false, reason
	}
	passed, failed, skipped := seg.passed, seg.failed, seg.skipped
	var out strings.Builder
	fmt.Fprintf(&out, "Go test -v: PASS (%d passed, 0 failed, %d skipped) in %s\n", passed, skipped, seg.time)
	if seg.subtests > 0 {
		fmt.Fprintf(&out, "Subtests: %d passed or skipped under those tests\n", seg.subtests)
	}
	if seg.pkg != "" {
		fmt.Fprintf(&out, "Package: %s\n", seg.pkg)
	}
	if seg.folded > 0 {
		fmt.Fprintf(&out, "%d passed tests logged output (folded)\n", seg.folded)
	}
	if len(seg.kept) > 0 {
		out.WriteString("Kept verbatim:\n")
		for _, l := range seg.kept {
			out.WriteString(l + "\n")
		}
	}
	compacted := []byte(out.String())
	if len(compacted) >= len(input) {
		return input, false, "no_compression_gain"
	}
	return compacted, true, fmt.Sprintf("compressed go test -v results (%d passed, %d failed, %d skipped)", passed, failed, skipped)
}

// goVerboseSeg is what scanGoVerbose accounts for in one package's transcript.
type goVerboseSeg struct {
	passed, failed, skipped, subtests, folded int
	kept                                      []string
	pkg, time, tail                           string
}

// scanGoVerbose reads one package's `go test -v` transcript (its tests, the
// PASS marker and the ok line) and returns what it accounts for, or a reason
// when any line is not one it can fold. allowEmpty accepts a package that ran
// no test (a bare PASS and ok line).
func scanGoVerbose(lines []string, allowEmpty bool) (goVerboseSeg, string) {
	var seg goVerboseSeg
	var blocks [][]string
	// open holds the blocks of tests that started and have no outcome yet; a
	// parallel test pauses, so several can be open. cur is the one the next
	// output line belongs to (the last RUN, CONT or NAME header).
	open := map[string][]string{}
	cur := ""
	var passed, failed, skipped, subtests int
	var kept []string
	folded := 0
	var packageName, packageTime, packageTail, lastName string
	terminal := ""
	seenPassMarker := false

	for _, line := range lines {
		if line == "" {
			continue
		}
		if cur != "" {
			if m := reGoVerboseSubRun.FindStringSubmatch(line); m != nil && strings.HasPrefix(m[1], cur+"/") {
				open[cur] = append(open[cur], line)
				continue
			}
		} else if m := reGoVerboseSubOutcome.FindStringSubmatch(line); m != nil && lastName != "" && strings.HasPrefix(m[2], lastName+"/") {
			// Go prints a parent's own outcome first and its indented subtest
			// outcomes right after it.
			if m[1] == "FAIL" {
				return seg, "go_test_verbose_failure_preserved"
			}
			subtests++
			continue
		}
		if m := reGoVerboseRun.FindStringSubmatch(line); m != nil {
			if _, dup := open[m[1]]; dup {
				return seg, "go_test_verbose_unexpected_nested_or_duplicate_run"
			}
			cur = m[1]
			open[cur] = []string{line}
			continue
		}
		if m := reGoVerbosePauseCont.FindStringSubmatch(line); m != nil {
			if top := openOwner(open, m[1]); top != "" {
				cur = top
				open[top] = append(open[top], line)
				continue
			}
		}
		if m := reGoVerboseOutcome.FindStringSubmatch(line); m != nil {
			block, isOpen := open[m[2]]
			if !isOpen {
				return seg, "go_test_verbose_unmatched_test_outcome"
			}
			blocks = append(blocks, append(block, line))
			delete(open, m[2])
			if cur == m[2] {
				cur = ""
			}
			lastName = m[2]
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
		if cur != "" {
			// Captured test output is only folded once the outcome is known.
			open[cur] = append(open[cur], line)
			continue
		}
		if line == "PASS" || line == "FAIL" {
			if terminal != "" || line == "PASS" && failed > 0 || line == "FAIL" && failed == 0 {
				return seg, "go_test_verbose_ambiguous_terminal_marker"
			}
			terminal = line
			seenPassMarker = line == "PASS"
			continue
		}
		if m := reGoVerboseOK.FindStringSubmatch(line); m != nil {
			if terminal != "PASS" || packageName != "" {
				return seg, "go_test_verbose_unexpected_package_summary"
			}
			packageName, packageTime = m[1], m[2]
			packageTail = strings.TrimSpace(line[strings.Index(line, m[2])+len(m[2]):])
			continue
		}
		if m := reGoVerboseFAIL.FindStringSubmatch(line); m != nil {
			if terminal != "FAIL" || packageName != "" {
				return seg, "go_test_verbose_unexpected_package_summary"
			}
			packageName, packageTime = m[1], m[2]
			continue
		}
		// A line outside every test block that we cannot account for stays
		// verbatim, unless it carries a failure signal: then the run is raw.
		if goFoldDanger.MatchString(line) {
			return seg, "go_test_verbose_danger_line_preserved"
		}
		kept = append(kept, line)
	}
	if len(open) != 0 || len(blocks) == 0 && !allowEmpty || packageName == "" || terminal == "" {
		return seg, "go_test_verbose_incomplete_transcript"
	}
	if terminal == "PASS" && (failed != 0 || !seenPassMarker) || terminal == "FAIL" && failed == 0 {
		return seg, "go_test_verbose_result_mismatch"
	}
	if terminal == "PASS" && len(blocks) != passed+skipped || terminal == "FAIL" && len(blocks) != passed+failed+skipped {
		return seg, "go_test_verbose_result_mismatch"
	}
	if terminal != "PASS" || failed > 0 {
		return seg, "go_test_verbose_failure_preserved"
	}
	seg.passed, seg.failed, seg.skipped, seg.subtests, seg.folded = passed, failed, skipped, subtests, folded
	seg.kept, seg.pkg, seg.time, seg.tail = kept, packageName, packageTime, packageTail
	return seg, ""
}

// openOwner returns the open top-level test a PAUSE, CONT or NAME header
// names, as the test itself or one of its subtests; "" when none is open.
func openOwner(open map[string][]string, name string) string {
	if _, ok := open[name]; ok {
		return name
	}
	for top := range open {
		if strings.HasPrefix(name, top+"/") {
			return top
		}
	}
	return ""
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
