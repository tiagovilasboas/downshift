// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

//go:build !windows

package execwrap

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/tiagovilasboas/downshift/internal/compressor"
)

// fakeBin writes an executable shell script named name into a fresh PATH
// directory, so Run sees argv[0] == "go" or "git" without the real tools.
func fakeBin(t *testing.T, name, script string) {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte("#!/bin/sh\n"+script+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func run(t *testing.T, argv []string, compact bool) (Result, string, string) {
	t.Helper()
	var out, errb bytes.Buffer
	r := Run(argv, Options{Stdin: strings.NewReader(""), Stdout: &out, Stderr: &errb, Compact: compact})
	return r, out.String(), errb.String()
}

// passingSuite is a realistic all-ok run: enough packages that the compressor
// summary is strictly shorter than the input.
var passingSuite = func() string {
	var b strings.Builder
	for _, p := range []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j"} {
		b.WriteString("ok  \tgithub.com/x/internal/" + p + "\t0.1s\n")
	}
	return b.String()
}()

// compactedWant is what a compacted run prints: the compressor's own summary
// plus the marker line.
func compactedWant(raw string) string {
	out := compressor.CompressExit([]byte(raw), compressor.ModeSafe, 0).Output
	return string(out) + Marker(len(raw), len(out))
}

func TestRun_ExitCodesAndCompaction(t *testing.T) {
	cases := []struct {
		name       string
		bin        string
		script     string
		argv       []string
		compact    bool
		wantCode   int
		wantClass  string
		wantOut    string
		wantErr    string
		compacted  bool
		wantFamily Family
	}{
		{
			name: "passing go test compacts", bin: "go",
			script: "printf '" + strings.ReplaceAll(passingSuite, "\t", "\\t") + "'",
			argv:   []string{"go", "test", "./..."}, compact: true,
			wantCode: 0, wantClass: ExitClassZero, wantOut: compactedWant(passingSuite), compacted: true, wantFamily: FamilyGoTest,
		},
		{
			name: "kill switch keeps raw", bin: "go",
			script: "printf '" + strings.ReplaceAll(passingSuite, "\t", "\\t") + "'",
			argv:   []string{"go", "test", "./..."}, compact: false,
			wantCode: 0, wantClass: ExitClassZero, wantOut: passingSuite, wantFamily: FamilyGoTest,
		},
		{
			name: "failing go test stays raw", bin: "go",
			script: "printf -- '--- FAIL: TestX (0.00s)\\nFAIL\\n'; exit 1",
			argv:   []string{"go", "test", "./..."}, compact: true,
			wantCode: 1, wantClass: ExitClassNonZero, wantOut: "--- FAIL: TestX (0.00s)\nFAIL\n", wantFamily: FamilyGoTest,
		},
		{
			name: "exit 0 with stderr stays raw", bin: "go",
			script: "printf '" + strings.ReplaceAll(passingSuite, "\t", "\\t") + "'; echo 'warning: x' >&2",
			argv:   []string{"go", "test", "./..."}, compact: true,
			wantCode: 0, wantClass: ExitClassZero, wantOut: passingSuite, wantErr: "warning: x\n", wantFamily: FamilyGoTest,
		},
		{
			name: "clean git status compacts", bin: "git",
			script: "printf 'On branch main\\nYour branch is up to date with origin/main.\\n\\nnothing to commit, working tree clean\\n'",
			argv:   []string{"git", "status"}, compact: true,
			wantCode: 0, wantClass: ExitClassZero, wantOut: "On branch main (working tree clean)\n" + Marker(len("On branch main\nYour branch is up to date with origin/main.\n\nnothing to commit, working tree clean\n"), len("On branch main (working tree clean)\n")), compacted: true, wantFamily: FamilyGitStatus,
		},
		{
			name: "dirty git status stays raw", bin: "git",
			script: "printf 'On branch main\\nChanges not staged for commit:\\n\\tmodified:   a.go\\n'",
			argv:   []string{"git", "status"}, compact: true,
			wantCode: 0, wantClass: ExitClassZero, wantOut: "On branch main\nChanges not staged for commit:\n\tmodified:   a.go\n", wantFamily: FamilyGitStatus,
		},
		{
			name: "git status output under a go test argv is not compacted", bin: "go",
			script: "printf 'On branch main\\nnothing to commit, working tree clean\\n'",
			argv:   []string{"go", "test"}, compact: true,
			wantCode: 0, wantClass: ExitClassZero, wantOut: "On branch main\nnothing to commit, working tree clean\n", wantFamily: FamilyGoTest,
		},
		{
			name: "stdout and stderr stay separate", bin: "go",
			script: "echo out; echo err >&2",
			argv:   []string{"go", "test"}, compact: true,
			wantCode: 0, wantClass: ExitClassZero, wantOut: "out\n", wantErr: "err\n", wantFamily: FamilyGoTest,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fakeBin(t, tc.bin, tc.script)
			r, out, errOut := run(t, tc.argv, tc.compact)
			if r.ExitCode != tc.wantCode || r.ExitClass != tc.wantClass {
				t.Fatalf("exit = %d/%s, want %d/%s (stderr %q)", r.ExitCode, r.ExitClass, tc.wantCode, tc.wantClass, errOut)
			}
			if out != tc.wantOut {
				t.Fatalf("stdout = %q, want %q", out, tc.wantOut)
			}
			if errOut != tc.wantErr {
				t.Fatalf("stderr = %q, want %q", errOut, tc.wantErr)
			}
			if r.Compacted != tc.compacted || r.Family != tc.wantFamily {
				t.Fatalf("compacted=%v family=%q, want %v %q", r.Compacted, r.Family, tc.compacted, tc.wantFamily)
			}
			if r.EmittedBytes != int64(len(out)+len(errOut)) {
				t.Fatalf("EmittedBytes = %d, want %d", r.EmittedBytes, len(out)+len(errOut))
			}
			if !r.Compacted && r.RawBytes != r.EmittedBytes {
				t.Fatalf("raw %d != emitted %d without compaction", r.RawBytes, r.EmittedBytes)
			}
		})
	}
}

func TestRun_NotFound127(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	r, out, errOut := run(t, []string{"go", "test"}, true)
	if r.ExitCode != ExitNotFound || r.ExitClass != ExitClassNotFound {
		t.Fatalf("exit = %d/%s, want 127/not_found", r.ExitCode, r.ExitClass)
	}
	if out != "" || !strings.Contains(errOut, "go: command not found") {
		t.Fatalf("stdout=%q stderr=%q", out, errOut)
	}
}

// exec is not a generic runner: anything outside the allowlist is refused
// with 126 and nothing is spawned (the marker file would exist otherwise).
func TestRun_RefusesNonAllowlisted(t *testing.T) {
	dir := t.TempDir()
	marker := filepath.Join(dir, "ran")
	fakeBin(t, "tool", "touch "+marker)
	fakeBin(t, "downshift", "touch "+marker)
	for _, argv := range [][]string{
		{"tool"},
		{"rm", "-rf", marker},
		{"downshift", "exec", "--", "go", "test"}, // recursion
		{"/usr/bin/git", "status"},
		{"git", "log"},
		{"go", "build"},
		{"go"},
	} {
		r, out, errOut := run(t, argv, true)
		if r.ExitCode != ExitCannotExecute || r.ExitClass != ExitClassRefused || r.Family != FamilyNone {
			t.Fatalf("%v: result = %+v", argv, r)
		}
		if out != "" || !strings.Contains(errOut, "refused") {
			t.Fatalf("%v: stdout=%q stderr=%q", argv, out, errOut)
		}
		if _, err := os.Stat(marker); err == nil {
			t.Fatalf("%v: a program was spawned", argv)
		}
	}
}

func TestRun_NoArgv(t *testing.T) {
	r, _, errOut := run(t, nil, true)
	if r.ExitCode != 2 || errOut == "" {
		t.Fatalf("exit = %d stderr=%q", r.ExitCode, errOut)
	}
}

func TestRun_StdinInherited(t *testing.T) {
	fakeBin(t, "go", "cat")
	var out bytes.Buffer
	r := Run([]string{"go", "test"}, Options{Stdin: strings.NewReader("hello\n"), Stdout: &out, Stderr: &bytes.Buffer{}, Compact: true})
	if r.ExitCode != 0 || out.String() != "hello\n" {
		t.Fatalf("exit=%d out=%q", r.ExitCode, out.String())
	}
}

func TestRun_NonUTF8Passthrough(t *testing.T) {
	// Bytes 0xff 0xfe plus a NUL: the compressor refuses them, output is identical.
	fakeBin(t, "go", "printf 'ok  x 0.1s\\nok  y 0.1s\\n\\377\\376\\000\\n'")
	r, out, _ := run(t, []string{"go", "test"}, true)
	want := "ok  x 0.1s\nok  y 0.1s\n\xff\xfe\x00\n"
	if r.Compacted || out != want {
		t.Fatalf("compacted=%v out=%q", r.Compacted, out)
	}
}

func TestRun_LargeOutputStreamsRaw(t *testing.T) {
	// More than MaxProcessableBytes of passing lines: held bytes flush, the
	// rest streams, and the output is byte-identical and not compacted.
	n := compressor.MaxProcessableBytes/len("ok  github.com/x/pkg 0.1s\n") + 100
	fakeBin(t, "go", "i=0; while [ $i -lt "+itoa(n)+" ]; do echo 'ok  github.com/x/pkg 0.1s'; i=$((i+1)); done")
	r, out, _ := run(t, []string{"go", "test"}, true)
	want := strings.Repeat("ok  github.com/x/pkg 0.1s\n", n)
	if r.Compacted || out != want {
		t.Fatalf("compacted=%v len(out)=%d want %d", r.Compacted, len(out), len(want))
	}
	if r.RawBytes != int64(len(want)) || r.EmittedBytes != int64(len(want)) {
		t.Fatalf("raw=%d emitted=%d want %d", r.RawBytes, r.EmittedBytes, len(want))
	}
}

func TestRun_SignalForwarded(t *testing.T) {
	fakeBin(t, "go", "exec sleep 10")
	sig := make(chan os.Signal, 1)
	var out, errb bytes.Buffer
	go func() {
		time.Sleep(300 * time.Millisecond)
		sig <- syscall.SIGTERM
	}()
	start := time.Now()
	r := Run([]string{"go", "test"}, Options{Stdin: strings.NewReader(""), Stdout: &out, Stderr: &errb, Compact: true, Signals: sig})
	if r.ExitCode != 128+int(syscall.SIGTERM) || r.ExitClass != ExitClassSignal {
		t.Fatalf("exit = %d/%s, want %d/signal", r.ExitCode, r.ExitClass, 128+int(syscall.SIGTERM))
	}
	if time.Since(start) > 5*time.Second {
		t.Fatal("signal was not forwarded: child ran to completion")
	}
}

func itoa(n int) string {
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	if len(b) == 0 {
		return "0"
	}
	return string(b)
}

// TestRun_GoTestOverTwoSecondsCompacts verifies that commands taking longer than 2s
// (the previous HoldBound limit) are held until exit 0 and successfully compacted,
// rather than prematurely streaming raw due to time.
func TestRun_GoTestOverTwoSecondsCompacts(t *testing.T) {
	var suite strings.Builder
	for _, p := range []string{"a", "b", "c", "d", "e", "f", "g", "h", "i"} {
		suite.WriteString("ok  \tgithub.com/x/pkg/" + p + "\t0.1s\n")
	}
	suite.WriteString("?   \tgithub.com/x/pkg/notest1\t[no test files]\n")
	raw := suite.String()

	fakeBin(t, "go", "sleep 2.1; printf '"+strings.ReplaceAll(raw, "\t", "\\t")+"'")
	r, out, errb := run(t, []string{"go", "test", "./..."}, true)
	if r.ExitCode != 0 || r.ExitClass != ExitClassZero {
		t.Fatalf("exit = %d/%s, want 0/zero", r.ExitCode, r.ExitClass)
	}
	if !r.Compacted {
		t.Fatalf("command taking >2s was not compacted (prematurely aborted hold): %+v\nout:\n%s", r, out)
	}
	if !strings.Contains(out, "9 packages ok, 1 without tests") {
		t.Fatalf("expected summary in output: %q", out)
	}
	if !strings.Contains(out, "[downshift exec: compacted") {
		t.Fatalf("missing compaction marker in output: %q", out)
	}
	if errb != "" {
		t.Fatalf("expected empty stderr, got %q", errb)
	}
	if r.EmittedBytes >= r.RawBytes {
		t.Fatalf("emitted %d >= raw %d", r.EmittedBytes, r.RawBytes)
	}
}

// TestRun_GoTestOverTwoSecondsIncrementalCompacts proves that incremental stdout writes
// separated by more than 2 seconds are not prematurely flushed by the timer.
func TestRun_GoTestOverTwoSecondsIncrementalCompacts(t *testing.T) {
	part1 := "ok  \tgithub.com/x/pkg/a\t0.1s\nok  \tgithub.com/x/pkg/b\t0.1s\n"
	part2 := "ok  \tgithub.com/x/pkg/c\t0.1s\nok  \tgithub.com/x/pkg/d\t0.1s\nok  \tgithub.com/x/pkg/e\t0.1s\n?   \tgithub.com/x/pkg/notest\t[no test files]\n"

	// First chunk emitted immediately, second chunk emitted after 2.1s delay.
	// Under the old 2s HoldBound, part1 would have triggered the timer and flushed at 2s.
	fakeBin(t, "go", "printf '"+strings.ReplaceAll(part1, "\t", "\\t")+"'; sleep 2.1; printf '"+strings.ReplaceAll(part2, "\t", "\\t")+"'")
	r, out, errb := run(t, []string{"go", "test", "./..."}, true)
	if r.ExitCode != 0 || r.ExitClass != ExitClassZero {
		t.Fatalf("exit = %d/%s, want 0/zero", r.ExitCode, r.ExitClass)
	}
	if !r.Compacted {
		t.Fatalf("incremental write with >2s gap was not compacted: %+v\nout:\n%s", r, out)
	}
	if !strings.Contains(out, "5 packages ok, 1 without tests") {
		t.Fatalf("expected summary in output: %q", out)
	}
	if !strings.Contains(out, "[downshift exec: compacted") {
		t.Fatalf("expected compaction marker: %q", out)
	}
	if errb != "" {
		t.Fatalf("expected empty stderr, got %q", errb)
	}
}

// TestRun_GoTestOverTwoSecondsFailStaysRaw proves that a slow command (>2s)
// that fails (exit 1) remains 100% raw and is never compacted.
func TestRun_GoTestOverTwoSecondsFailStaysRaw(t *testing.T) {
	raw := "ok  \tgithub.com/x/pkg/a\t0.1s\n--- FAIL: TestFoo (0.01s)\nFAIL\nFAIL\tgithub.com/x/pkg/b\t0.02s\n?   \tgithub.com/x/pkg/c\t[no test files]\n"
	fakeBin(t, "go", "sleep 2.1; printf '"+strings.ReplaceAll(raw, "\t", "\\t")+"'; exit 1")
	r, out, _ := run(t, []string{"go", "test", "./..."}, true)
	if r.ExitCode != 1 || r.ExitClass != ExitClassNonZero {
		t.Fatalf("exit = %d/%s, want 1/nonzero", r.ExitCode, r.ExitClass)
	}
	if r.Compacted {
		t.Fatalf("failing command >2s must not be compacted: %+v", r)
	}
	if out != raw {
		t.Fatalf("output = %q, want raw %q", out, raw)
	}
	if r.RawBytes != r.EmittedBytes {
		t.Fatalf("raw %d != emitted %d", r.RawBytes, r.EmittedBytes)
	}
}

// TestRun_PytestVitestExecution validates the execwrap execution, compaction on clean exit 0,
// and byte-for-byte raw preservation on exit != 0 or errors for pytest and vitest.
func TestRun_PytestVitestExecution(t *testing.T) {
	pytestClean := "============================= test session starts ==============================\nrootdir: /tmp\ncollected 25 items\n\ntest_a.py .....\ntest_b.py .....\ntest_c.py .....\ntest_d.py .....\ntest_e.py .....\n\n============================== 25 passed in 0.42s ==============================\n"
	pytestFail := "============================= test session starts ==============================\nrootdir: /tmp\ncollected 25 items\n\ntest_a.py ..F..\n\n=================================== FAILURES ===================================\n_________________________________ test_charge __________________________________\nAssertionError\n========================= 1 failed, 24 passed in 0.48s =========================\n"

	vitestClean := "\n RUN  v1.6.0 /Users/tiago.boas/web\n\n ✓ test/a.test.ts (5)\n ✓ test/b.test.ts (5)\n ✓ test/c.test.ts (5)\n ✓ test/d.test.ts (5)\n ✓ test/e.test.ts (5)\n\n Test Files  5 passed (5)\n      Tests  25 passed (25)\n   Start at  19:20:00\n   Duration  412ms\n"
	vitestFail := "\n RUN  v1.6.0 /Users/tiago.boas/web\n\n ❯ test/b.test.ts (5)\n   × test_charge\n\n⎯⎯⎯⎯⎯⎯⎯ Failed Tests 1 ⎯⎯⎯⎯⎯⎯⎯\n\n FAIL  test/b.test.ts > test_charge\nAssertionError: expected 100 to be 110\n\n Test Files  1 failed | 4 passed (5)\n      Tests  1 failed | 24 passed (25)\n"

	cases := []struct {
		name       string
		bin        string
		script     string
		argv       []string
		compact    bool
		wantCode   int
		wantClass  string
		wantOut    string
		wantErr    string
		compacted  bool
		wantFamily Family
	}{
		{
			name: "passing pytest compacts", bin: "pytest",
			script: "printf '" + strings.ReplaceAll(pytestClean, "\n", "\\n") + "'",
			argv:   []string{"pytest"}, compact: true,
			wantCode: 0, wantClass: ExitClassZero, wantOut: compactedWant(pytestClean), compacted: true, wantFamily: Family("pytest"),
		},
		{
			name: "pytest kill switch keeps raw", bin: "pytest",
			script: "printf '" + strings.ReplaceAll(pytestClean, "\n", "\\n") + "'",
			argv:   []string{"pytest"}, compact: false,
			wantCode: 0, wantClass: ExitClassZero, wantOut: pytestClean, wantFamily: Family("pytest"),
		},
		{
			name: "failing pytest stays raw", bin: "pytest",
			script: "printf '" + strings.ReplaceAll(pytestFail, "\n", "\\n") + "'; exit 1",
			argv:   []string{"pytest"}, compact: true,
			wantCode: 1, wantClass: ExitClassNonZero, wantOut: pytestFail, wantFamily: Family("pytest"),
		},
		{
			name: "clean pytest with stderr stays raw", bin: "pytest",
			script: "printf '" + strings.ReplaceAll(pytestClean, "\n", "\\n") + "'; echo 'pytest warning' >&2",
			argv:   []string{"pytest"}, compact: true,
			wantCode: 0, wantClass: ExitClassZero, wantOut: pytestClean, wantErr: "pytest warning\n", wantFamily: Family("pytest"),
		},
		{
			name: "passing vitest compacts", bin: "vitest",
			script: "printf '" + strings.ReplaceAll(vitestClean, "\n", "\\n") + "'",
			argv:   []string{"vitest", "run"}, compact: true,
			wantCode: 0, wantClass: ExitClassZero, wantOut: compactedWant(vitestClean), compacted: true, wantFamily: Family("vitest"),
		},
		{
			name: "vitest kill switch keeps raw", bin: "vitest",
			script: "printf '" + strings.ReplaceAll(vitestClean, "\n", "\\n") + "'",
			argv:   []string{"vitest", "run"}, compact: false,
			wantCode: 0, wantClass: ExitClassZero, wantOut: vitestClean, wantFamily: Family("vitest"),
		},
		{
			name: "failing vitest stays raw", bin: "vitest",
			script: "printf '" + strings.ReplaceAll(vitestFail, "\n", "\\n") + "'; exit 1",
			argv:   []string{"vitest", "run"}, compact: true,
			wantCode: 1, wantClass: ExitClassNonZero, wantOut: vitestFail, wantFamily: Family("vitest"),
		},
		{
			name: "clean vitest with stderr stays raw", bin: "vitest",
			script: "printf '" + strings.ReplaceAll(vitestClean, "\n", "\\n") + "'; echo 'vitest warning' >&2",
			argv:   []string{"vitest", "run"}, compact: true,
			wantCode: 0, wantClass: ExitClassZero, wantOut: vitestClean, wantErr: "vitest warning\n", wantFamily: Family("vitest"),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fakeBin(t, tc.bin, tc.script)
			r, out, errOut := run(t, tc.argv, tc.compact)
			if r.ExitCode != tc.wantCode || r.ExitClass != tc.wantClass {
				t.Fatalf("exit = %d/%s, want %d/%s (stderr %q)", r.ExitCode, r.ExitClass, tc.wantCode, tc.wantClass, errOut)
			}
			if out != tc.wantOut {
				t.Fatalf("stdout = %q, want %q", out, tc.wantOut)
			}
			if errOut != tc.wantErr {
				t.Fatalf("stderr = %q, want %q", errOut, tc.wantErr)
			}
			if r.Compacted != tc.compacted || r.Family != tc.wantFamily {
				t.Fatalf("compacted=%v family=%q, want %v %q", r.Compacted, r.Family, tc.compacted, tc.wantFamily)
			}
			if r.EmittedBytes != int64(len(out)+len(errOut)) {
				t.Fatalf("EmittedBytes = %d, want %d", r.EmittedBytes, len(out)+len(errOut))
			}
			if !r.Compacted && r.RawBytes != r.EmittedBytes {
				t.Fatalf("raw %d != emitted %d without compaction", r.RawBytes, r.EmittedBytes)
			}
		})
	}
}
