// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const corpusDir = "testdata/corpus"

// nonZeroExits covers ordinary failures, timeout(1) and signal-style codes.
var nonZeroExits = []int{1, 2, 124, 130, 137, 255}

// rawFixtures must never change, whatever the exit code (including unknown -1).
var rawFixtures = []string{
	// go test: failures, panics, races, build and vet errors.
	"gotest_fail_mixed_ok.txt",
	"gotest_fail_mixed_no_test_files.txt",
	"gotest_fail_summary_only.txt",
	"gotest_panic.txt",
	"gotest_panic_mixed_no_test_files.txt",
	"gotest_race.txt",
	"gotest_race_no_fail_line.txt",
	"gotest_build_failed.txt",
	"gotest_vet_error.txt",
	// go test: not all-ok lines, so not clearly clean.
	"gotest_no_tests_to_run.txt",
	"gotest_verbose.txt",
	"gotest_pass_with_log_line.txt",
	"gotest_crlf.txt",
	"gotest_truncated_last_line.txt",
	// git status with anything to look at.
	"gitstatus_untracked.txt",
	"gitstatus_modified_staged.txt",
	"gitstatus_conflict.txt",
	"gitstatus_rebase_in_progress.txt",
	"gitstatus_detached_dirty.txt",
	"gitstatus_ansi.txt",
	"gitstatus_porcelain_v2.txt",
	"gitstatus_short.txt",
	// D1: repeated lines must not trigger log dedup on a failing run.
	"gotest_fail_repeated_log.txt",
	// D8: no final newline means possibly truncated.
	"gotest_pass_no_trailing_newline.txt",
	// D6: a summary may never be longer than the input.
	"gotest_two_short_pkgs.txt",
	// D3/D4: a clean tree still carries facts the summary would drop.
	"gitstatus_clean_ahead.txt",
	"gitstatus_clean_behind.txt",
	"gitstatus_clean_diverged.txt",
	"gitstatus_clean_upstream_gone.txt",
	"gitstatus_clean_bisecting.txt",
	"gitstatus_merge_concluding.txt",
	"gitstatus_clean_stash.txt",
	"gitstatus_clean_detached.txt",
	// D5: CRLF would be rewritten with a stray carriage return.
	"gitstatus_clean_crlf.txt",
	// git log: only the signed and oneline shapes stay raw; default-format logs
	// compact (see TestGitLogCompaction and the MANIFEST).
	"gitlog_signed.txt",
	"gitlog_oneline.txt",
	// encodings.
	"non_utf8.bin",
	"binary_nul.bin",
	// pytest and vitest: failures and tracebacks stay raw.
	"pytest_fail.txt",
	"vitest_fail.txt",
	// playwright, cargo test and tsc: failures and errors stay raw.
	"playwright_fail.txt",
	"cargo_fail.txt",
	"tsc_fail.txt",
	"jest_fail.txt",
	"next_build_warning.txt",
	"next_build_truncated.txt",
	"next_build_unknown_output.txt",
	"phpunit_fail_actionable.txt",
	// flutter test and dart test: failures, test print output and pub get
	// chatter stay raw.
	"flutter_fail.txt",
	"flutter_print_output.txt",
	"flutter_pub_get_noise.txt",
	"maven_fail_test.txt",
	"maven_warning.txt",
	"maven_downloads.txt",
	"maven_skipped.txt",
	"maven_multimodule.txt",
	"gradle_fail.txt",
	"gradle_kotlin_warning.txt",
	"gradle_deprecation.txt",
	"gradle_test_output.txt",
}

// cleanFixture is output that may be compacted on exit 0. facts are substrings
// that must survive compaction.
type cleanFixture struct {
	file  string
	facts []string
}

var cleanFixtures = []cleanFixture{
	{"gotest_pass_coverage.txt", []string{"PASS", "12 packages", "internal/billing", "65.9%", "cmd/widget", "100.0%"}},
	{"gotest_pass_cached.txt", []string{"PASS", "6 packages", "internal/config"}},
	{"gotest_pass_plain.txt", []string{"PASS", "12 packages", "internal/store"}},
	{"gotest_no_test_files.txt", []string{"PASS", "2 packages ok", "1 without tests", "cmd/widget", "internal/api", "internal/auth"}},
	{"gotest_pass_mixed_no_test_files.txt", []string{"PASS", "6 packages ok", "3 without tests", "cmd/dsmon-server", "internal/adapt", "80.5%", "internal/session", "internal/testrace"}},
	{"gitstatus_clean_uptodate.txt", []string{"On branch main", "working tree clean"}},
	{"gitstatus_clean_no_upstream.txt", []string{"On branch feature/rate-limit", "working tree clean"}},
	{"pytest_pass_clean.txt", []string{"25 passed"}},
	// A wrapped progress line, and -v with ids that hold spaces, brackets and angle brackets.
	{"pytest_pass_wrapped.txt", []string{"120 passed"}},
	{"pytest_pass_clean_verbose_ids.txt", []string{"18 passed"}},
	{"vitest_pass_clean.txt", []string{"25 passed"}},
	// Real vitest 5 bytes: colour codes, skipped and todo counts, a skipped file.
	{"vitest_v5_colour_clean.txt", []string{"2 passed, 3 skipped, 1 todo across 3 files in 74ms"}},
	// Real vitest 4 shapes with the Node and jsdom notices merged in from stderr:
	// each distinct notice survives once, with its count.
	{"vitest_v4_notices_clean.txt", []string{"90 passed across 12 files in 9.14s", "[DEP0205] DeprecationWarning",
		"ExperimentalWarning: localStorage is not available", "[repeated 5 times]", "Not implemented: navigation to another Document"}},
	{"playwright_pass_clean.txt", []string{"25 passed"}},
	{"cargo_pass_clean.txt", []string{"25 passed"}},
	{"tsc_pass_clean.txt", []string{"0 errors"}},
	{"jest_pass_clean.txt", []string{"25 passed", "3 suites", "1 suite skipped", "1 todo"}},
	{"next_build_pass.txt", []string{"5 routes", "8.2s"}},
	{"gradle_pass_build.txt", []string{"BUILD SUCCESSFUL in 12s", "14 actionable tasks: 9 executed, 5 up-to-date", "14 tasks"}},
	{"gradle_pass_uptodate.txt", []string{"BUILD SUCCESSFUL in 1s", "14 actionable tasks: 14 up-to-date", "Starting a Gradle Daemon"}},
	{"gradle_pass_test.txt", []string{"BUILD SUCCESSFUL in 8s", "6 actionable tasks: 2 executed, 4 up-to-date"}},
	{"phpunit_pass_default.txt", []string{"58 tests", "143 assertions", "00:00.019", "12.00 MB"}},
	{"flutter_pass_compact.txt", []string{"PASS", "3 passed", "1 skipped", "00:02"}},
	{"flutter_pass_expanded.txt", []string{"PASS", "3 passed", "1 skipped"}},
	{"dart_pass_compact.txt", []string{"PASS", "3 passed", "1 skipped"}},
	{"dart_pass_expanded.txt", []string{"PASS", "3 passed", "1 skipped"}},
	{"maven_pass_test.txt", []string{"inventory 1.4.0", "surefire:test", "Tests run: 13, Failures: 0, Errors: 0, Skipped: 0", "BUILD SUCCESS", "Total time:  2.915 s"}},
	{"maven_pass_package.txt", []string{"jar:jar", "Tests run: 13, Failures: 0, Errors: 0, Skipped: 0", "BUILD SUCCESS", "Total time:  4.302 s"}},
	// Colour codes are not content: a coloured clean run compacts like a plain
	// one, and its summary is plain text (colour.go).
	{"gotest_ansi.txt", []string{"PASS", "5 packages ok", "cmd/widget", "internal/cache"}},
	{"dart_pass_ansi.txt", []string{"PASS", "3 passed", "1 skipped"}},
	{"maven_ansi.txt", []string{"surefire:test", "Tests run: 13, Failures: 0, Errors: 0, Skipped: 0", "BUILD SUCCESS", "Total time:  2.915 s"}},
	{"gradle_ansi.txt", []string{"BUILD SUCCESSFUL in 12s", "14 actionable tasks: 9 executed, 5 up-to-date"}},
}

func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(corpusDir, name))
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return b
}

// allFixtures returns every file in the corpus directory, so new fixtures are
// covered by the invariants without being listed.
func allFixtures(t *testing.T) []string {
	t.Helper()
	ents, err := os.ReadDir(corpusDir)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, e := range ents {
		if !e.IsDir() && !strings.HasPrefix(e.Name(), ".") {
			out = append(out, e.Name())
		}
	}
	if len(out) < len(rawFixtures)+len(cleanFixtures) {
		t.Fatalf("corpus has %d files, fewer than the listed fixtures", len(out))
	}
	return out
}
