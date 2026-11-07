// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package docscheck_test

import (
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

var envName = regexp.MustCompile(`DOWNSHIFT_[A-Z0-9]+(?:_[A-Z0-9]+)*`)

// envNames returns the DOWNSHIFT_* names in text; a trailing underscore or
// star (a prefix such as DOWNSHIFT_*) names no variable and is skipped.
func envNames(text string) []string {
	var out []string
	for _, loc := range envName.FindAllStringIndex(text, -1) {
		tail := text[loc[1]:]
		if strings.HasPrefix(tail, "_") || strings.HasPrefix(tail, "*") {
			continue
		}
		out = append(out, text[loc[0]:loc[1]])
	}
	return out
}

// retired matches a line that says the variable is gone, which is the one
// honest reason to name a variable the code no longer reads.
var retired = regexp.MustCompile(`(?i)removed|removidos?|no longer|retired|not read|ignored`)

// envOnlyInDocs lists variables the docs name that no source file mentions.
func envOnlyInDocs(lines []docLine, code map[string]bool) []string {
	var bad []string
	for _, l := range lines {
		if retired.MatchString(l.Text) {
			continue
		}
		for _, n := range envNames(l.Text) {
			if !code[n] {
				bad = append(bad, l.File+":"+strconv.Itoa(l.N)+" "+n)
			}
		}
	}
	return bad
}

// envOnlyInCode lists variables a source file reads that no guarded doc names.
func envOnlyInCode(lines []docLine, code map[string]bool) []string {
	documented := map[string]bool{}
	for _, l := range lines {
		for _, n := range envNames(l.Text) {
			documented[n] = true
		}
	}
	var bad []string
	for n := range code {
		if !documented[n] {
			bad = append(bad, n)
		}
	}
	sort.Strings(bad)
	return bad
}

// codeEnv is every DOWNSHIFT_* name in the non-test Go, shell and TypeScript
// sources: an os.Getenv call, a LookupEnv call or a constant all mention it.
func codeEnv(t *testing.T, root string) map[string]bool {
	t.Helper()
	code := map[string]bool{}
	for _, f := range sourceFiles(t, root, []string{".go", ".sh", ".ts", ".ps1", ".cmd"}, false) {
		for _, n := range envNames(mustRead(t, f)) {
			code[n] = true
		}
	}
	return code
}

// testEnv is every DOWNSHIFT_* name a test file reads, such as the bench's
// DOWNSHIFT_BENCH_RTK. A doc may name these. This package's own tests are
// skipped: they spell stale names on purpose to prove the guards bite.
func testEnv(t *testing.T, root string) map[string]bool {
	t.Helper()
	read := map[string]bool{}
	own := filepath.Join(root, "internal", "docscheck") + string(filepath.Separator)
	for _, f := range sourceFiles(t, root, []string{".go"}, true) {
		if strings.HasPrefix(f, own) {
			continue
		}
		for _, n := range envNames(mustRead(t, f)) {
			read[n] = true
		}
	}
	return read
}

// Every DOWNSHIFT_* variable a doc names is read by the code, and every one
// the code reads is documented.
func TestDocumentedEnvVarsMatchTheCode(t *testing.T) {
	root := repoRoot(t)
	lines, code := guardedLines(t, root), codeEnv(t, root)
	// CHANGELOG.md records variables that were later removed.
	var current []docLine
	for _, l := range lines {
		if l.File != "CHANGELOG.md" {
			current = append(current, l)
		}
	}
	readable := map[string]bool{}
	for n := range code {
		readable[n] = true
	}
	for n := range testEnv(t, root) {
		readable[n] = true
	}
	for _, b := range envOnlyInDocs(current, readable) {
		t.Errorf("%s is documented but no code reads it; remove it or mark the line %s", b, historyMarker)
	}
	for _, n := range envOnlyInCode(lines, withoutTestOnly(code)) {
		t.Errorf("%s is read by the code but documented nowhere; add it to the environment table in docs/reference/cli.md", n)
	}
}

// testOnly are read by developer tooling or are local variables of a
// maintainer script, never by the shipped binary, so no user doc names them.
var testOnly = []string{
	"DOWNSHIFT_HOOK_TEST_P", // cmd/devcheck: parallelism of the local test gate
	"DOWNSHIFT_TEST_TIMING", // internal/testenv: slow-test report in tests
}

func withoutTestOnly(code map[string]bool) map[string]bool {
	out := map[string]bool{}
	for n := range code {
		out[n] = true
	}
	for _, n := range testOnly {
		delete(out, n)
	}
	return out
}

func TestEnvGuardBites(t *testing.T) {
	lines := []docLine{{"x.md", 1, "set DOWNSHIFT_LOG_LEVEL or DOWNSHIFT_EXEC and DOWNSHIFT_*"}}
	if got := envOnlyInDocs(lines, map[string]bool{"DOWNSHIFT_EXEC": true}); len(got) != 1 {
		t.Errorf("doc-only: got %v", got)
	}
	if got := envOnlyInCode(lines, map[string]bool{"DOWNSHIFT_EXEC": true, "DOWNSHIFT_NEW": true}); len(got) != 1 {
		t.Errorf("code-only: got %v", got)
	}
}
