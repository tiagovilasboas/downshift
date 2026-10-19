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

const passingSuite = "ok  \tgithub.com/x/a\t0.1s\nok  \tgithub.com/x/b\t(cached)\nok  \tgithub.com/x/c\t0.2s\n"

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
			wantCode: 0, wantClass: ExitClassZero, wantOut: "Go test: PASS (3 packages ok)\n", compacted: true, wantFamily: FamilyGoTest,
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
			wantCode: 0, wantClass: ExitClassZero, wantOut: "On branch main (working tree clean)\n", compacted: true, wantFamily: FamilyGitStatus,
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
