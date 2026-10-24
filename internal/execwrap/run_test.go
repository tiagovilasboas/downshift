// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

//go:build !windows

package execwrap

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/tiagovilasboas/downshift/internal/compressor"
	"github.com/tiagovilasboas/downshift/internal/testenv"
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
			name: "git log is wrapped and passes through raw", bin: "git",
			script: "printf 'commit 0123456789abcdef0123456789abcdef01234567\\nAuthor: A <a@example.com>\\n\\n    Init\\n'",
			argv:   []string{"git", "log"}, compact: true,
			wantCode: 0, wantClass: ExitClassZero, wantOut: "commit 0123456789abcdef0123456789abcdef01234567\nAuthor: A <a@example.com>\n\n    Init\n", wantFamily: FamilyGitLog,
		},
		{
			name: "git push progress on stderr stays raw", bin: "git",
			script: "echo 'Everything up-to-date' >&2",
			argv:   []string{"git", "push"}, compact: true,
			wantCode: 0, wantClass: ExitClassZero, wantErr: "Everything up-to-date\n", wantFamily: FamilyGitPush,
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

func TestRun_JestCompactionAndFailOpen(t *testing.T) {
	clean := "PASS src/a.test.ts\nPASS src/b.test.ts\n\nTest Suites: 2 passed, 2 total\nTests:       25 passed, 25 total\nSnapshots:   0 total\nTime:        1.234 s\n"
	fail := "FAIL src/a.test.ts\n\nTest Suites: 1 failed, 1 total\nTests:       1 failed, 24 passed, 25 total\n"
	cases := []struct {
		name, raw, want string
		code            int
	}{
		{"passing Jest compacts", clean, compactedWant(clean), 0},
		{"failing Jest stays raw", fail, fail, 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			script := "cat <<'EOF'\n" + tc.raw + "EOF\n"
			if tc.code != 0 {
				script += "exit 1\n"
			}
			fakeBin(t, "jest", script)
			r, out, errOut := run(t, []string{"jest"}, true)
			if r.ExitCode != tc.code || out != tc.want || errOut != "" {
				t.Fatalf("result=%+v stdout=%q stderr=%q, want code=%d stdout=%q", r, out, errOut, tc.code, tc.want)
			}
			if tc.code == 0 && (!r.Compacted || r.Family != FamilyJest) {
				t.Fatalf("passing Jest should compact under its family: %+v", r)
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
		{"git", "--version"},
		{"git", "-c", "core.pager=x", "log"},
		{"sudo", "rm", "-rf", "/"},
		{"eval", "echo", "test"},
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
	if !strings.Contains(out, "[downshift compact: compacted") {
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
	if !strings.Contains(out, "[downshift compact: compacted") {
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

// TestRun_PlaywrightCargoTscExecution validates the execwrap execution, compaction on clean exit 0,
// and byte-for-byte raw preservation on exit != 0 or errors for playwright, cargo test, and tsc.
func TestRun_PlaywrightCargoTscExecution(t *testing.T) {
	playwrightClean := "Running 25 tests using 4 workers\n\n  ✓  1 [chromium] › e2e/auth.spec.ts:10:5 › login flow (1.2s)\n  ✓  2 [chromium] › e2e/auth.spec.ts:25:5 › logout flow (850ms)\n  ✓  3 [chromium] › e2e/billing.spec.ts:12:5 › invoice generation (1.5s)\n  ✓  4 [chromium] › e2e/billing.spec.ts:30:5 › stripe checkout (2.1s)\n  ✓  5 [chromium] › e2e/dashboard.spec.ts:8:5 › load metrics (920ms)\n\n  25 passed (4.2s)\n"
	playwrightFail := "Running 25 tests using 4 workers\n\n  ✓  1 [chromium] › e2e/auth.spec.ts:10:5 › login flow (1.2s)\n  ✘  2 [chromium] › e2e/billing.spec.ts:30:5 › stripe checkout (2.1s)\n\n  1) [chromium] › e2e/billing.spec.ts:30:5 › stripe checkout ───────────────────────────────────────────\n\n    Error: Timed out 5000ms waiting for expect(locator).toBeVisible()\n\n    Locator: getByText('Payment Successful')\n    Expected: visible\n    Received: <element(s) not found>\n\n      32 |     await page.getByRole('button', { name: 'Pay' }).click();\n    > 33 |     await expect(page.getByText('Payment Successful')).toBeVisible();\n         |                                                        ^\n      34 |   });\n\n        at /Users/tiago.boas/web/e2e/billing.spec.ts:33:56\n\n  1 failed\n    [chromium] › e2e/billing.spec.ts:30:5 › stripe checkout ────────────────────────────────────────────\n  24 passed (6.5s)\n"

	cargoClean := "   Compiling downshift-core v0.1.0 (/Users/tiago.boas/rust/downshift)\n    Finished `test` profile [unoptimized + debuginfo] target(s) in 0.85s\n     Running unittests src/lib.rs (target/debug/deps/downshift_core-a1b2c3d4)\n\nrunning 25 tests\ntest auth::tests::test_token_validation ... ok\ntest auth::tests::test_session_expiry ... ok\ntest billing::tests::test_invoice_calc ... ok\ntest billing::tests::test_tax_rate ... ok\ntest config::tests::test_load_defaults ... ok\ntest store::tests::test_cache_hit ... ok\ntest store::tests::test_cache_miss ... ok\ntest utils::tests::test_sanitize ... ok\ntest utils::tests::test_slugify ... ok\ntest engine::tests::test_parse_input ... ok\n\ntest result: ok. 25 passed; 0 failed; 0 ignored; 0 measured; 0 filtered out; finished in 0.14s\n"
	cargoFail := "   Compiling downshift-core v0.1.0 (/Users/tiago.boas/rust/downshift)\n    Finished `test` profile [unoptimized + debuginfo] target(s) in 0.92s\n     Running unittests src/lib.rs (target/debug/deps/downshift_core-a1b2c3d4)\n\nrunning 25 tests\ntest auth::tests::test_token_validation ... ok\ntest billing::tests::test_invoice_calc ... FAILED\ntest config::tests::test_load_defaults ... ok\ntest store::tests::test_cache_hit ... ok\n\nfailures:\n\n---- billing::tests::test_invoice_calc stdout ----\nthread 'billing::tests::test_invoice_calc' panicked at src/billing.rs:42:9:\nassertion `left == right` failed\n  left: 100\n right: 110\nnote: run with `RUST_BACKTRACE=1` environment variable to display a backtrace\n\nfailures:\n    billing::tests::test_invoice_calc\n\ntest result: FAILED. 24 passed; 1 failed; 0 ignored; 0 measured; 0 filtered out; finished in 0.15s\n\nerror: test failed, to rerun pass `--lib`\n"

	tscClean := "[19:20:00] Projects in this build: \n    * tsconfig.json\n\n[19:20:01] Building project '/Users/tiago.boas/project/tsconfig.json'...\n[19:20:02] Found 0 errors. Watching for file changes.\n"
	tscFail := "src/components/Button.tsx:12:7 - error TS2322: Type 'string' is not assignable to type 'number'.\n\n12 const width: number = \"100px\";\n         ~~~~~\n\nsrc/utils/format.ts:45:10 - error TS2304: Cannot find name 'unknownVar'.\n\n45   return unknownVar;\n            ~~~~~~~~~~\n\nFound 2 errors in 2 files.\n\nErrors  Files\n     1  src/components/Button.tsx:12\n     1  src/utils/format.ts:45\n"

	catScript := func(body string, code int, stderr string) string {
		s := "cat << 'EOF'\n" + body + "EOF\n"
		if stderr != "" {
			s += "echo '" + stderr + "' >&2\n"
		}
		if code != 0 {
			s += fmt.Sprintf("exit %d\n", code)
		}
		return s
	}

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
			name: "passing playwright compacts", bin: "playwright",
			script: catScript(playwrightClean, 0, ""),
			argv:   []string{"playwright", "test"}, compact: true,
			wantCode: 0, wantClass: ExitClassZero, wantOut: compactedWant(playwrightClean), compacted: true, wantFamily: Family("playwright"),
		},
		{
			name: "playwright kill switch keeps raw", bin: "playwright",
			script: catScript(playwrightClean, 0, ""),
			argv:   []string{"playwright", "test"}, compact: false,
			wantCode: 0, wantClass: ExitClassZero, wantOut: playwrightClean, wantFamily: Family("playwright"),
		},
		{
			name: "failing playwright stays raw", bin: "playwright",
			script: catScript(playwrightFail, 1, ""),
			argv:   []string{"playwright", "test"}, compact: true,
			wantCode: 1, wantClass: ExitClassNonZero, wantOut: playwrightFail, wantFamily: Family("playwright"),
		},
		{
			name: "clean playwright with stderr stays raw", bin: "playwright",
			script: catScript(playwrightClean, 0, "playwright warning"),
			argv:   []string{"playwright", "test"}, compact: true,
			wantCode: 0, wantClass: ExitClassZero, wantOut: playwrightClean, wantErr: "playwright warning\n", wantFamily: Family("playwright"),
		},
		{
			name: "passing cargo test compacts", bin: "cargo",
			script: catScript(cargoClean, 0, ""),
			argv:   []string{"cargo", "test"}, compact: true,
			wantCode: 0, wantClass: ExitClassZero, wantOut: compactedWant(cargoClean), compacted: true, wantFamily: Family("cargo-test"),
		},
		{
			name: "cargo test kill switch keeps raw", bin: "cargo",
			script: catScript(cargoClean, 0, ""),
			argv:   []string{"cargo", "test"}, compact: false,
			wantCode: 0, wantClass: ExitClassZero, wantOut: cargoClean, wantFamily: Family("cargo-test"),
		},
		{
			name: "failing cargo test stays raw", bin: "cargo",
			script: catScript(cargoFail, 1, ""),
			argv:   []string{"cargo", "test"}, compact: true,
			wantCode: 1, wantClass: ExitClassNonZero, wantOut: cargoFail, wantFamily: Family("cargo-test"),
		},
		{
			name: "clean cargo test with stderr stays raw", bin: "cargo",
			script: catScript(cargoClean, 0, "cargo warning"),
			argv:   []string{"cargo", "test"}, compact: true,
			wantCode: 0, wantClass: ExitClassZero, wantOut: cargoClean, wantErr: "cargo warning\n", wantFamily: Family("cargo-test"),
		},
		{
			name: "passing tsc compacts", bin: "tsc",
			script: catScript(tscClean, 0, ""),
			argv:   []string{"tsc"}, compact: true,
			wantCode: 0, wantClass: ExitClassZero, wantOut: compactedWant(tscClean), compacted: true, wantFamily: Family("tsc"),
		},
		{
			name: "tsc kill switch keeps raw", bin: "tsc",
			script: catScript(tscClean, 0, ""),
			argv:   []string{"tsc"}, compact: false,
			wantCode: 0, wantClass: ExitClassZero, wantOut: tscClean, wantFamily: Family("tsc"),
		},
		{
			name: "failing tsc stays raw", bin: "tsc",
			script: catScript(tscFail, 1, ""),
			argv:   []string{"tsc"}, compact: true,
			wantCode: 1, wantClass: ExitClassNonZero, wantOut: tscFail, wantFamily: Family("tsc"),
		},
		{
			name: "clean tsc with stderr stays raw", bin: "tsc",
			script: catScript(tscClean, 0, "tsc warning"),
			argv:   []string{"tsc"}, compact: true,
			wantCode: 0, wantClass: ExitClassZero, wantOut: tscClean, wantErr: "tsc warning\n", wantFamily: Family("tsc"),
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

func TestRun_CypressPassingRunCompacts(t *testing.T) {
	const output = "  (Run Starting)\n  (Run Finished)\n  │ Tests: 2\n  │ Passing: 2\n  │ Failing: 0\n  │ Pending: 0\n  │ Skipped: 0\n  │ Duration: 1 second\n  │ Spec Ran: cypress/e2e/accounting.cy.ts\n"
	fakeBin(t, "cypress", "cat <<'EOF'\n"+output+"EOF")
	r, stdout, stderr := run(t, []string{"cypress", "run"}, true)
	if r.ExitCode != 0 || r.ExitClass != ExitClassZero {
		t.Fatalf("exit = %d/%s, want 0/zero; stderr=%q", r.ExitCode, r.ExitClass, stderr)
	}
	if !r.Compacted || r.Family != FamilyCypress {
		t.Fatalf("compacted=%v family=%q, want true cypress", r.Compacted, r.Family)
	}
	if !strings.Contains(stdout, "Cypress: PASS (2 tests") || strings.Contains(stdout, "Run Starting") {
		t.Fatalf("unexpected Cypress output: %q", stdout)
	}
}

func TestRun_NextBuildAndPHPUnitCompact(t *testing.T) {
	for _, tc := range []struct {
		name, bin, command, fixture, want string
		family                            Family
	}{
		{"Next.js build", "next", "next build", "next_build_pass.txt", "Next.js Build: SUCCESS (5 routes: 3 app, 2 pages; shared First Load JS 96.7 kB in 8.2s)", FamilyNextBuild},
		{"Gradle", "gradle", "gradle build", "gradle_pass_build.txt", "gradle: 14 tasks", FamilyGradle},
		{"PHPUnit", "phpunit", "phpunit", "phpunit_pass_default.txt", "PHPUnit: PASS (58 tests, 143 assertions", FamilyPHPUnit},
		{"Flutter test", "flutter", "flutter test", "flutter_pass_compact.txt", "dart/flutter test: PASS (3 passed, 1 skipped", FamilyFlutterTest},
		{"Dart test", "dart", "dart test --no-color", "dart_pass_compact.txt", "dart/flutter test: PASS (3 passed, 1 skipped", FamilyDartTest},
		{"Maven", "mvn", "mvn test", "maven_pass_test.txt", "[INFO] BUILD SUCCESS", FamilyMaven},
	} {
		t.Run(tc.name, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join("..", "compressor", "testdata", "corpus", tc.fixture))
			if err != nil {
				t.Fatal(err)
			}
			fakeBin(t, tc.bin, "cat <<'EOF'\n"+string(data)+"EOF\n")
			argv := strings.Fields(tc.command)
			r, stdout, stderr := run(t, argv, true)
			if r.ExitCode != 0 || !r.Compacted || r.Family != tc.family || stderr != "" || !strings.Contains(stdout, tc.want) {
				t.Fatalf("result=%+v stdout=%q stderr=%q", r, stdout, stderr)
			}
		})
	}
}

func TestRun_WaitsForObserveHookUnderDeadline(t *testing.T) {
	fakeBin(t, "go", "printf 'not a go test output\\n'")
	prevHook, prevDeadline := ObserveUncompactedHook, ObserveHookDeadline
	defer func() { ObserveUncompactedHook, ObserveHookDeadline = prevHook, prevDeadline }()

	var done atomic.Bool
	ObserveUncompactedHook = func(argv []string, sample []byte, family Family) {
		time.Sleep(50 * time.Millisecond)
		if string(sample) == "not a go test output\n" && family == FamilyGoTest {
			done.Store(true)
		}
	}
	r, out, _ := run(t, []string{"go", "test"}, true)
	if r.ExitCode != 0 || r.Compacted || out != "not a go test output\n" {
		t.Fatalf("exit=%d compacted=%v out=%q", r.ExitCode, r.Compacted, out)
	}
	if !done.Load() {
		t.Error("Run returned before the hook finished")
	}

	// A hook that overruns the deadline neither blocks Run nor changes the exit code.
	ObserveHookDeadline = 20 * time.Millisecond
	ObserveUncompactedHook = func([]string, []byte, Family) { time.Sleep(2 * time.Second) }
	start := time.Now()
	r, _, _ = run(t, []string{"go", "test"}, true)
	// The hook sleeps 2 s: finishing under 2 s proves Run did not wait for it
	// (always checked); the tighter 1 s bound is a performance budget.
	elapsed := time.Since(start)
	if r.ExitCode != 0 || elapsed >= 2*time.Second {
		t.Fatalf("overrunning hook changed the result: exit=%d elapsed=%s", r.ExitCode, elapsed)
	}
	testenv.CheckBudget(t, "Run with an overrunning hook", elapsed, time.Second)

	// A panicking hook is swallowed.
	ObserveUncompactedHook = func([]string, []byte, Family) { panic("boom") }
	if r, _, _ = run(t, []string{"go", "test"}, true); r.ExitCode != 0 {
		t.Fatalf("panicking hook changed the exit code: %d", r.ExitCode)
	}
}

// TestRun_ContainerExecKeepsExitCode runs a fake docker that prints a PHPUnit
// transcript. A zero exit compacts under the inner family; a non-zero exit
// keeps the raw output and the real exit code.
func TestRun_ContainerExecKeepsExitCode(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "compressor", "testdata", "corpus", "phpunit_pass_default.txt"))
	if err != nil {
		t.Fatal(err)
	}
	argv := strings.Fields("docker compose exec -T app vendor/bin/phpunit")

	fakeBin(t, "docker", "cat <<'EOF'\n"+string(data)+"EOF\nexit 0")
	r, stdout, stderr := run(t, argv, true)
	if r.ExitCode != 0 || !r.Compacted || r.Family != FamilyPHPUnit || stderr != "" || !strings.Contains(stdout, "PHPUnit: PASS (58 tests, 143 assertions") {
		t.Fatalf("pass: result=%+v stdout=%q stderr=%q", r, stdout, stderr)
	}

	fakeBin(t, "docker", "cat <<'EOF'\n"+string(data)+"EOF\nexit 3")
	r, stdout, _ = run(t, argv, true)
	if r.ExitCode != 3 || r.Compacted || r.Family != FamilyPHPUnit || stdout != string(data) {
		t.Fatalf("fail: result=%+v stdout=%q", r, stdout)
	}
}
