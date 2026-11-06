// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import (
	"fmt"
	"strings"
)

// goVerboseEntry is one package's slice of a multi-package `go test -v`
// transcript: folded into counts, a package without tests, or kept verbatim.
type goVerboseEntry struct {
	seg     goVerboseSeg
	folded  bool
	noTests string
	raw     []string
}

// countGoVerbosePackages counts the package result lines (ok, FAIL, no test
// files) of a transcript.
func countGoVerbosePackages(input []byte) int {
	n := 0
	for _, l := range strings.Split(string(input), "\n") {
		if reGoVerboseOK.MatchString(l) || reGoVerboseFAIL.MatchString(l) || reGoTestNoTest.MatchString(l) {
			n++
		}
	}
	return n
}

// splitGoVerbosePackages cuts a `go test -v ./...` transcript at each package
// result line. A package that scans clean is folded; any other stretch of
// lines (a failure, a danger word, output it cannot account for) is kept
// verbatim, so nothing it says is dropped.
func splitGoVerbosePackages(lines []string) []goVerboseEntry {
	var entries []goVerboseEntry
	var cur []string
	flush := func() {
		if strings.TrimSpace(strings.Join(cur, "")) != "" {
			entries = append(entries, goVerboseEntry{raw: cur})
		}
		cur = nil
	}
	for _, line := range lines {
		cur = append(cur, line)
		switch {
		case reGoVerboseOK.MatchString(line) || reGoVerboseFAIL.MatchString(line):
			if seg, reason := scanGoVerbose(cur, true); reason == "" {
				entries = append(entries, goVerboseEntry{seg: seg, folded: true})
				cur = nil
			} else {
				flush()
			}
		case reGoTestNoTest.MatchString(line):
			if m := reGoTestNoTest.FindStringSubmatch(line); strings.TrimSpace(strings.Join(cur[:len(cur)-1], "")) == "" {
				entries = append(entries, goVerboseEntry{noTests: m[1]})
				cur = nil
			} else {
				flush()
			}
		}
	}
	flush()
	return entries
}

// parseGoVerboseMulti compacts a `go test -v` run over several packages. Each
// package whose transcript is clean folds into one line; the rest stays
// verbatim in order. The run stays raw when no package folds or the result is
// not shorter.
func parseGoVerboseMulti(input []byte) ([]byte, bool, string) {
	entries := splitGoVerbosePackages(strings.Split(string(input), "\n"))
	var names []string
	var packages, passed, skipped, noTests, verbatim int
	for _, e := range entries {
		switch {
		case e.folded:
			packages++
			passed += e.seg.passed
			skipped += e.seg.skipped
			names = append(names, e.seg.pkg)
		case e.noTests != "":
			noTests++
			names = append(names, e.noTests)
		default:
			verbatim++
		}
	}
	if packages == 0 {
		return input, false, "go_test_verbose_no_package_folded"
	}
	prefix := commonPathPrefix(names)
	var b strings.Builder
	if verbatim == 0 {
		fmt.Fprintf(&b, "Go test -v: PASS (%d packages ok, %d passed, %d skipped, %d without tests)", packages, passed, skipped, noTests)
	} else {
		fmt.Fprintf(&b, "Go test -v: %d packages folded (%d passed, %d skipped); %d other stretches kept verbatim", packages, passed, skipped, verbatim)
	}
	if prefix != "" {
		fmt.Fprintf(&b, " under %s", prefix)
	}
	b.WriteString("\n")
	for _, e := range entries {
		writeGoVerboseEntry(&b, e, prefix)
	}
	out := b.String()
	if len(out) >= len(input) {
		return input, false, "no_compression_gain"
	}
	return []byte(out), true, fmt.Sprintf("compressed go test -v results over %d packages (%d passed)", packages, passed)
}

func writeGoVerboseEntry(b *strings.Builder, e goVerboseEntry, prefix string) {
	switch {
	case e.folded:
		fmt.Fprintf(b, "ok %s %s", strings.TrimPrefix(e.seg.pkg, prefix), e.seg.time)
		if e.seg.tail != "" {
			b.WriteString(" " + e.seg.tail)
		}
		fmt.Fprintf(b, " (%d passed", e.seg.passed)
		if e.seg.skipped > 0 {
			fmt.Fprintf(b, ", %d skipped", e.seg.skipped)
		}
		if e.seg.folded > 0 {
			fmt.Fprintf(b, ", %d with logs folded", e.seg.folded)
		}
		b.WriteString(")\n")
		for _, l := range e.seg.kept {
			b.WriteString(l + "\n")
		}
	case e.noTests != "":
		b.WriteString("? " + strings.TrimPrefix(e.noTests, prefix) + " [no test files]\n")
	default:
		for _, l := range e.raw {
			if l != "" {
				b.WriteString(l + "\n")
			}
		}
	}
}
