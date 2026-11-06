// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor_test

// The invariants of the lossless families. An invariant takes a clean fixture
// and the output the wrapper emits for it, and returns the signal lines the
// output lost (empty when nothing is lost).

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/tiagovilasboas/downshift/internal/compressor"
)

type invariant func(in, out []byte) (lost []string)

// zeroCount matches a line that reports "nothing wrong" ("0 errors",
// "Failing: 0"). A summary may fold it into its verdict, so it is not a signal.
var zeroCount = regexp.MustCompile(`(?i)\b0\s+(errors?|failed|failures?|failing|warnings?)\b|(errors?|failed|failures?|failing|warnings?)\s*[:=]\s*0\b`)

// processID is a per-process id in a Node notice; the same notice from several
// workers differs only by it, and the compaction folds them with a count.
var processID = regexp.MustCompile(`\(node:\d+\)`)

// dangerSignals returns the failure lines of a runner transcript: every line
// with error, fail, warning or panic in it that is not a zero count. Colour is
// not content, and indentation is not either.
func dangerSignals(in []byte) []string {
	var out []string
	for _, l := range splitLines(in) {
		l = strings.TrimSpace(processID.ReplaceAllString(colourCodes.ReplaceAllString(l, ""), "(node:N)"))
		if l != "" && dangerWords.MatchString(l) && !zeroCount.MatchString(l) {
			out = append(out, l)
		}
	}
	return out
}

// hasNumber reports whether out carries n as a whole number.
func hasNumber(out []byte, n int) bool {
	return regexp.MustCompile(`(^|[^0-9])` + strconv.Itoa(n) + `([^0-9]|$)`).Match(out)
}

func matchNumbers(in []byte, re *regexp.Regexp) []int {
	var ns []int
	for _, m := range re.FindAllSubmatch(in, -1) {
		if n, err := strconv.Atoi(string(m[1])); err == nil {
			ns = append(ns, n)
		}
	}
	return ns
}

// passTotal derives the pass count a reader trusts from the input. ok is false
// when the transcript has no such count (the invariant then checks only the
// failure lines).
type passTotal func(in []byte) (n int, ok bool)

func sumOf(re *regexp.Regexp) passTotal {
	return func(in []byte) (int, bool) {
		ns, sum := matchNumbers(in, re), 0
		for _, n := range ns {
			sum += n
		}
		return sum, len(ns) > 0
	}
}

func maxOf(re *regexp.Regexp) passTotal {
	return func(in []byte) (int, bool) {
		ns, top := matchNumbers(in, re), 0
		for _, n := range ns {
			top = max(top, n)
		}
		return top, len(ns) > 0
	}
}

func lastOf(re *regexp.Regexp) passTotal {
	return func(in []byte) (int, bool) {
		ns := matchNumbers(in, re)
		if len(ns) == 0 {
			return 0, false
		}
		return ns[len(ns)-1], true
	}
}

var (
	reOKLine   = regexp.MustCompile(`(?m)^ok\s`)
	reTopPass  = regexp.MustCompile(`(?m)^--- PASS:`)
	rePassedN  = regexp.MustCompile(`(\d+) passed`)
	reCypress  = regexp.MustCompile(`Passing:\s+(\d+)`)
	rePHPUnit  = regexp.MustCompile(`OK \((\d+) tests`)
	reFlutter  = regexp.MustCompile(`\+(\d+)`)
	reTscFound = regexp.MustCompile(`Found (\d+) errors?`)
	reRspecN   = regexp.MustCompile(`(\d+) examples?,`)
	reDotnetN  = regexp.MustCompile(`Passed:\s+(\d+)`)
)

// goTestTotal is the number of top-level tests that passed in a verbose run, or
// else of packages that said ok.
func goTestTotal(in []byte) (int, bool) {
	if n := len(reTopPass.FindAll(in, -1)); n > 0 {
		return n, true
	}
	n := len(reOKLine.FindAll(in, -1))
	return n, n > 0
}

// runnerInvariant: every failure line and the pass total of the transcript
// survive the compaction.
func runnerInvariant(total passTotal) invariant {
	return func(in, out []byte) []string {
		var lost []string
		for _, l := range dangerSignals(in) {
			if !strings.Contains(processID.ReplaceAllString(string(out), "(node:N)"), l) {
				lost = append(lost, "failure line: "+l)
			}
		}
		if n, ok := total(in); ok && !hasNumber(out, n) {
			lost = append(lost, fmt.Sprintf("total: %d", n))
		}
		return lost
	}
}

var reGrepMatch = regexp.MustCompile(`^(.+?):(\d+):(?:\d+:)?(.*)$`)

// grepInvariant: every file:line and its match text survive. The compacted form
// groups by file, so a match is found as "<line>: <text>" under its file header.
func grepInvariant(in, out []byte) []string {
	var lost []string
	for _, l := range splitLines(in) {
		m := reGrepMatch.FindStringSubmatch(l)
		if m == nil {
			continue
		}
		text := strings.TrimSpace(m[3])
		grouped := m[2] + ": " + text
		if !strings.Contains(string(out), m[1]) || !strings.Contains(string(out), grouped) && !strings.Contains(string(out), l) {
			lost = append(lost, m[1]+":"+m[2])
		}
	}
	return lost
}

// grepRowsInvariant: every path and its match text survive. The compacted form
// groups by path, so a match is found as its text, trimmed, under the path header.
func grepRowsInvariant(in, out []byte) []string {
	var lost []string
	for _, l := range splitLines(in) {
		path, text, ok := strings.Cut(l, ":")
		if !ok {
			continue
		}
		if !strings.Contains(string(out), path) || !strings.Contains(string(out), strings.TrimLeft(text, " \t")) {
			lost = append(lost, l)
		}
	}
	return lost
}

var reRubocopKept = regexp.MustCompile(`(?m)^(\S+?:\d+:\d+: [EF]: .*|\d+ files? inspected, .*)$`)

// rubocopInvariant: every danger line, every error and fatal offence line and
// the summary line survive verbatim.
func rubocopInvariant(in, out []byte) []string {
	lost := runnerInvariant(func([]byte) (int, bool) { return 0, false })(in, out)
	for _, l := range reRubocopKept.FindAllString(string(in), -1) {
		if !strings.Contains(string(out), l) {
			lost = append(lost, l)
		}
	}
	return lost
}

// invariants maps each lossless Format to its check.
var invariants = map[compressor.Format]invariant{
	compressor.FormatCargoTest:   runnerInvariant(sumOf(rePassedN)),
	compressor.FormatCypress:     runnerInvariant(lastOf(reCypress)),
	compressor.FormatFlutterTest: runnerInvariant(lastOf(reFlutter)),
	compressor.FormatGoTest:      runnerInvariant(goTestTotal),
	compressor.FormatGoTestRuns:  runnerInvariant(goTestTotal),
	compressor.FormatJest:        runnerInvariant(maxOf(rePassedN)),
	compressor.FormatPHPUnit:     runnerInvariant(lastOf(rePHPUnit)),
	compressor.FormatPlaywright:  runnerInvariant(maxOf(rePassedN)),
	compressor.FormatPytest:      runnerInvariant(maxOf(rePassedN)),
	compressor.FormatTSC:         runnerInvariant(lastOf(reTscFound)),
	compressor.FormatVitest:      runnerInvariant(maxOf(rePassedN)),
	compressor.FormatRspec:       runnerInvariant(lastOf(reRspecN)),
	compressor.FormatDotnetTest:  runnerInvariant(lastOf(reDotnetN)),
	compressor.FormatRubocop:     rubocopInvariant,
	compressor.FormatGrep:        grepInvariant,
	compressor.FormatGrepRows:    grepRowsInvariant,
}

// invariantAllowlist names the lossless families that have no invariant yet,
// each with the reason. A family listed here must not also have an invariant
// (the suite fails on a stale entry).
var invariantAllowlist = map[compressor.Format]string{
	compressor.FormatNextBuild:  "verdict + route count summary; no failure lines or pass totals to compare (TODO: route-count invariant)",
	compressor.FormatTurbo:      "task-count summary; clean output has no failure lines (TODO: successful/total invariant)",
	compressor.FormatNx:         "one-line success summary; no totals in the transcript",
	compressor.FormatNpmInstall: "install summary; the signal is the package count, not a failure line (TODO)",
	compressor.FormatPipInstall: "install summary; the signal is the package list, not a failure line (TODO)",
	compressor.FormatGitDiff:    "output keeps hunks verbatim; the invariant would be a diff line-set compare (TODO)",
	compressor.FormatGitShow:    "output keeps commit and hunks verbatim; line-set compare (TODO)",
	compressor.FormatGitRemote:  "clean hook and push transcripts; summary by design drops progress, no failure lines (TODO)",
	compressor.FormatPHPStan:    "clean run replaced by its verdict; no failure lines or totals to compare",
	compressor.FormatGitStatus:  "clean tree: branch line is the only signal and is kept by the family's own tests",
	compressor.FormatGhPr:       "field list, not a runner; no failure signal",
	compressor.FormatGhChecks:   "check list; a failing check is a preserve fixture, not a clean one (TODO)",
	compressor.FormatGhRunList:  "run list; a failing run is a preserve fixture, not a clean one (TODO)",
	compressor.FormatLs:         "name listing; every name is the signal, compared by the listing tests (TODO)",
	compressor.FormatFind:       "path tree; every path is the signal, compared by the listing tests (TODO)",
}
