// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package docscheck_test

import (
	"regexp"
	"strconv"
	"testing"
)

var (
	citedTest = regexp.MustCompile(`\bTest[A-Z]\w*`)
	testFunc  = regexp.MustCompile(`(?m)^func (Test[A-Z]\w*)\(`)
)

// citedTestsMissing returns the Test names in lines that no defined test has.
func citedTestsMissing(lines []docLine, defined map[string]bool) []string {
	var bad []string
	for _, l := range lines {
		for _, name := range citedTest.FindAllString(l.Text, -1) {
			if !defined[name] {
				bad = append(bad, l.File+":"+strconv.Itoa(l.N)+" "+name)
			}
		}
	}
	return bad
}

// Every test a doc cites exists as a func in some _test.go file. A quoted
// fixture or an old name goes on a line marked as history.
func TestDocsCiteOnlyExistingTests(t *testing.T) {
	root := repoRoot(t)
	defined := map[string]bool{}
	for _, f := range sourceFiles(t, root, []string{"_test.go"}, true) {
		for _, m := range testFunc.FindAllStringSubmatch(mustRead(t, f), -1) {
			defined[m[1]] = true
		}
	}
	for _, b := range citedTestsMissing(guardedLines(t, root), defined) {
		t.Errorf("%s: no such test; fix the doc or mark the line %s", b, historyMarker)
	}
}

func TestCitedTestGuardBites(t *testing.T) {
	lines := []docLine{{"x.md", 3, "see `TestNoSuchThing` and TestReal"}}
	got := citedTestsMissing(lines, map[string]bool{"TestReal": true})
	if len(got) != 1 {
		t.Fatalf("got %v, want only TestNoSuchThing", got)
	}
}
