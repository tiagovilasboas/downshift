// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift-ai/internal/compressor"
)

// Every line the program printed itself survives, in order; only the clean go
// test package lines fold into the summary.
func TestFoldGoTestRunsKeepsEveryOtherLine(t *testing.T) {
	for _, tc := range []struct {
		file string
		kept []string
	}{
		{"make_check_clean.txt", []string{"[1/4] gofmt -l .", "[3/4] go vet ./...  (GOOS=windows)", "[5/5] go test -count=1 ./...", "go test -count=1 ./..."}},
		{"git_push_hooks_clean.txt", []string{"lefthook  v2.2.2   hook:  pre-push", "[test-changed] ./cmd/widget ./internal/api",
			"✔ check-fast (2.89 seconds)", "✔ test-changed (68.20 seconds)", "To github.com:acme/widget.git",
			"   1a2b3c4..5d6e7f8  feature/cache -> feature/cache"}},
		{"git_commit_hooks_clean.txt", []string{"✔ tests (21.23 seconds)", "[feature/cache 5d6e7f8] Cache billing lookups",
			" 3 files changed, 42 insertions(+), 7 deletions(-)", " create mode 100644 internal/cache/lookup.go"}},
	} {
		in := readFixture(t, tc.file)
		r := compressor.FoldGoTestRuns(in)
		if !r.Applied || r.Format != compressor.FormatGoTestRuns || len(r.Output) >= len(in) {
			t.Fatalf("%s: applied=%v format=%q %d -> %d (%s)", tc.file, r.Applied, r.Format, len(in), len(r.Output), r.Reason)
		}
		if !bytes.Contains(r.Output, []byte("Go test: PASS (")) {
			t.Errorf("%s: no go test summary: %q", tc.file, r.Output)
		}
		last := -1
		for _, k := range tc.kept {
			i := bytes.Index(r.Output, []byte(k))
			if i < 0 || i < last {
				t.Errorf("%s: line %q lost or out of order:\n%s", tc.file, k, r.Output)
			}
			last = i
		}
		if bytes.Contains(r.Output, []byte("\x1b")) {
			t.Errorf("%s: summary kept escape codes", tc.file)
		}
	}
}

func TestFoldGoTestRunsLeavesRawWhatItCannotFold(t *testing.T) {
	ok := func(p string) string { return "ok  \texample.com/acme/" + p + "\t0.10s\n" }
	run := ok("a") + ok("b") + ok("c") + ok("d")
	for name, in := range map[string]string{
		"run too short":        "make test\n" + ok("a") + ok("b"),
		"no package lines":     "To github.com:acme/widget.git\n   1a2b3c4..5d6e7f8  main -> main\n",
		"carriage return":      "Writing objects:  50%\rdone\n" + run,
		"terminal escape":      "\x1b[2K" + run,
		"no final newline":     strings.TrimSuffix(run, "\n"),
		"tap lines are not go": "ok 1 adds\nok 2 subtracts\nok 3 divides\nok 4 multiplies\n",
	} {
		r := compressor.FoldGoTestRuns([]byte(in))
		if r.Applied || !bytes.Equal(r.Output, []byte(in)) {
			t.Errorf("%s: applied=%v reason=%q, want the input back", name, r.Applied, r.Reason)
		}
	}
}

// Output that carries a go test failure comes back untouched, so a
// "Go test: PASS" summary never sits next to a failure.
func TestFoldGoTestRunsKeepsFailuresRaw(t *testing.T) {
	ok := func(p string) string { return "ok  \texample.com/acme/" + p + "\t0.10s\n" }
	run := ok("a") + ok("b") + ok("c")
	for name, fail := range map[string]string{
		"failed test":       "--- FAIL: TestX (0.00s)\n",
		"failed subtest":    "    --- FAIL: TestX/case (0.00s)\n",
		"failed package":    "FAIL\texample.com/acme/x\t0.02s\n",
		"bare FAIL":         "FAIL\n",
		"panic":             "panic: runtime error: index out of range\n",
		"data race":         "WARNING: DATA RACE\n",
		"data race in line": "==================\nWARNING: DATA RACE\n",
	} {
		in := run + fail + ok("d") + ok("e") + ok("f")
		r := compressor.FoldGoTestRuns([]byte(in))
		if r.Applied || !bytes.Equal(r.Output, []byte(in)) || r.Reason != "go_test_runs_failure_preserved" {
			t.Errorf("%s: applied=%v reason=%q, want the input back", name, r.Applied, r.Reason)
		}
	}
}

// A line that is not a clean package line splits a run: each side folds on its
// own and the line stays between them. Words like "failed_count" are not
// failures.
func TestFoldGoTestRunsSplitsAroundOtherLines(t *testing.T) {
	ok := func(p string) string { return "ok  \texample.com/acme/" + p + "\t0.10s\n" }
	const mid = "failed_count=0 FAILED=none\n"
	in := ok("a") + ok("b") + ok("c") + mid + ok("d") + ok("e") + ok("f")
	r := compressor.FoldGoTestRuns([]byte(in))
	if !r.Applied {
		t.Fatalf("not applied: %s", r.Reason)
	}
	parts := strings.Split(string(r.Output), mid)
	if len(parts) != 2 || !strings.HasPrefix(parts[0], "Go test: PASS (3 packages ok)") || !strings.HasPrefix(parts[1], "Go test: PASS (3 packages ok)") {
		t.Fatalf("runs not folded on both sides of the line:\n%s", r.Output)
	}
}

// Folding the folded output again changes nothing.
func TestFoldGoTestRunsIsIdempotent(t *testing.T) {
	once := compressor.FoldGoTestRuns(readFixture(t, "make_check_clean.txt"))
	twice := compressor.FoldGoTestRuns(once.Output)
	if twice.Applied || !bytes.Equal(twice.Output, once.Output) {
		t.Fatalf("second fold changed the output: %q", twice.Reason)
	}
}
