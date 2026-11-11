// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import (
	"fmt"
	"strings"

	"github.com/tiagovilasboas/downshift-ai/internal/lazyre"
)

var (
	// reGoFailedRunMarker is a RUN/PAUSE/CONT/NAME header of one test.
	reGoFailedRunMarker = lazyre.New(`^=== (?:RUN|PAUSE|CONT|NAME)\s+(\S+)$`)
	// reGoFailedPass is the outcome line of a passed test or subtest.
	reGoFailedPass = lazyre.New(`^(\s*)--- PASS: (\S+) \([^()]*\)$`)
	// reGoFailedFail is the outcome line of a failed test or subtest.
	reGoFailedFail = lazyre.New(`^\s*--- FAIL: (\S+) \([^()]*\)$`)
	// reGoFailedOK is a passing package line without coverage data. A line
	// with "coverage:" is kept, since the percentage is information.
	reGoFailedOK = lazyre.New(`^ok\s+\S+\s+(?:\(cached\)|[0-9.]+s)$`)
	// reGoFailedSignal marks output that really is a failed go test run.
	reGoFailedSignal = lazyre.New(`(?m)^(?:\s*--- FAIL:|FAIL(?:\s|$)|\s*panic:|fatal error:|WARNING: DATA RACE)|\[(?:build|setup) failed\]`)
	// reGoFailedJSON is a `go test -json` event line.
	reGoFailedJSON = lazyre.New(`(?m)^\{"`)
	// reGoStdlibFunc matches stack frame function names originating in stdlib runtime or testing.
	reGoStdlibFunc = lazyre.New(`^(?:testing\.|runtime\.|created by (?:testing|runtime)\.)`)
	// reGoStdlibPath matches stack frame source paths originating in stdlib runtime or testing.
	reGoStdlibPath = lazyre.New(`^\t.*(?:/src/testing/|/src/runtime/|/testing/testing\.go:|/runtime/panic\.go:|/runtime/[a-zA-Z0-9_]+\.(?:go|s):)`)
)

// compactFailedGoTest drops passing noise, folds internal stdlib panic frames,
// and prepends one summary line (or omits it if the header eliminates the gain).
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

	kept = FoldGoStdlibFrames(kept)

	if okPkgs == 0 && passedTests == 0 && len(kept) == len(lines) {
		return "", false
	}

	var parts []string
	if okPkgs > 0 {
		if okPkgs == 1 {
			parts = append(parts, "ok: 1 pkg")
		} else {
			parts = append(parts, fmt.Sprintf("ok: %d pkgs", okPkgs))
		}
	}
	if passedTests > 0 {
		if passedTests == 1 {
			parts = append(parts, "1 passing test cut")
		} else {
			parts = append(parts, fmt.Sprintf("%d passing tests cut", passedTests))
		}
	}

	header := ""
	if len(parts) > 0 {
		header = "Go test: " + strings.Join(parts, ", ") + "\n"
	}

	body := strings.Join(kept, "\n") + "\n"
	full := header + body
	if len(full) < len(input) {
		return full, true
	}
	if header != "" && len(body) < len(input) {
		return body, true
	}
	return "", false
}

// isGoStdlibFrame reports whether the two-line stack frame belongs to Go internal testing or runtime packages.
func isGoStdlibFrame(fnLine, pathLine string) bool {
	return strings.HasPrefix(pathLine, "\t") &&
		reGoStdlibFunc.MatchString(fnLine) &&
		reGoStdlibPath.MatchString(pathLine)
}

// FoldGoStdlibFrames collapses contiguous internal stdlib panic frames into a single fold marker.
func FoldGoStdlibFrames(lines []string) []string {
	var out []string
	n := len(lines)
	for i := 0; i < n; {
		if i+1 < n && isGoStdlibFrame(lines[i], lines[i+1]) {
			for i+1 < n && isGoStdlibFrame(lines[i], lines[i+1]) {
				i += 2
			}
			out = append(out, "\t... [internal frames folded] ...")
			continue
		}
		out = append(out, lines[i])
		i++
	}
	return out
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
