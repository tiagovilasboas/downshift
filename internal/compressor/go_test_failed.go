// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	// reGoFailedRunMarker is a RUN/PAUSE/CONT/NAME header of one test.
	reGoFailedRunMarker = regexp.MustCompile(`^=== (?:RUN|PAUSE|CONT|NAME)\s+(\S+)$`)
	// reGoFailedPass is the outcome line of a passed test or subtest.
	reGoFailedPass = regexp.MustCompile(`^(\s*)--- PASS: (\S+) \([^()]*\)$`)
	// reGoFailedFail is the outcome line of a failed test or subtest.
	reGoFailedFail = regexp.MustCompile(`^\s*--- FAIL: (\S+) \([^()]*\)$`)
	// reGoFailedOK is a passing package line without coverage data. A line
	// with "coverage:" is kept, since the percentage is information.
	reGoFailedOK = regexp.MustCompile(`^ok\s+\S+\s+(?:\(cached\)|[0-9.]+s)$`)
	// reGoFailedSignal marks output that really is a failed go test run.
	reGoFailedSignal = regexp.MustCompile(`(?m)^(?:\s*--- FAIL:|FAIL(?:\s|$)|\s*panic:|fatal error:|WARNING: DATA RACE)|\[(?:build|setup) failed\]`)
	// reGoFailedJSON is a `go test -json` event line.
	reGoFailedJSON = regexp.MustCompile(`(?m)^\{"`)
)

// compactFailedGoTest drops passing noise and prepends one summary line.
func compactFailedGoTest(input string) (string, bool) {
	lines := strings.Split(strings.TrimSuffix(input, "\n"), "\n")

	passed := map[string]bool{}
	failed := map[string]bool{}
	for _, l := range lines {
		if m := reGoFailedPass.FindStringSubmatch(l); m != nil {
			passed[m[2]] = true
		} else if m := reGoFailedFail.FindStringSubmatch(l); m != nil {
			failed[m[1]] = true
		}
	}
	// A test that logged keeps its header lines so the log stays attributed.
	logged := map[string]bool{}
	for i, l := range lines {
		if i+1 >= len(lines) {
			break
		}
		next := lines[i+1]
		if m := reGoFailedRunMarker.FindStringSubmatch(l); m != nil && isIndentedLog(next, "") {
			logged[m[1]] = true
		} else if m := reGoFailedPass.FindStringSubmatch(l); m != nil && isIndentedLog(next, m[1]) {
			logged[m[2]] = true
		}
	}
	cuttable := func(name string) bool { return passed[name] && !failed[name] && !logged[name] }

	var kept []string
	okPkgs, passedTests := 0, 0
	for _, l := range lines {
		if reGoFailedOK.MatchString(l) {
			okPkgs++
			continue
		}
		if m := reGoFailedPass.FindStringSubmatch(l); m != nil && cuttable(m[2]) {
			passedTests++
			continue
		}
		if m := reGoFailedRunMarker.FindStringSubmatch(l); m != nil && cuttable(m[1]) {
			continue
		}
		kept = append(kept, l)
	}
	if okPkgs == 0 && passedTests == 0 && len(kept) == len(lines) {
		return "", false
	}

	var parts []string
	if okPkgs > 0 {
		parts = append(parts, fmt.Sprintf("ok: %d packages", okPkgs))
	}
	if passedTests > 0 {
		parts = append(parts, fmt.Sprintf("%d passing tests cut", passedTests))
	}
	var b strings.Builder
	if len(parts) > 0 {
		b.WriteString("Go test: " + strings.Join(parts, ", ") + " (failures kept verbatim)\n")
	}
	for _, l := range kept {
		b.WriteString(l + "\n")
	}
	return b.String(), true
}

// isIndentedLog reports whether line is output indented deeper than indent
// that is not itself a test marker.
func isIndentedLog(line, indent string) bool {
	if !strings.HasPrefix(line, indent) {
		return false
	}
	rest := line[len(indent):]
	if rest == "" || (rest[0] != ' ' && rest[0] != '\t') {
		return false
	}
	t := strings.TrimLeft(rest, " \t")
	return !strings.HasPrefix(t, "--- ") && !strings.HasPrefix(t, "=== ")
}
