// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

//go:build !windows

package execwrap

import (
	"strings"
	"testing"
)

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
			name: "exit 0 with a stderr note compacts and keeps the note", bin: "go",
			script: "printf '" + strings.ReplaceAll(passingSuite, "\t", "\\t") + "'; echo 'warning: x' >&2",
			argv:   []string{"go", "test", "./..."}, compact: true,
			wantCode: 0, wantClass: ExitClassZero, wantOut: compactedWant(passingSuite + "warning: x\n"), compacted: true, wantFamily: FamilyGoTest,
		},
		{
			name: "clean git status compacts", bin: "git",
			script: "printf 'On branch main\\nYour branch is up to date with origin/main.\\n\\nnothing to commit, working tree clean\\n'",
			argv:   []string{"git", "status"}, compact: true,
			wantCode: 0, wantClass: ExitClassZero, wantOut: "On branch main (working tree clean)\n" + Marker(len("On branch main\nYour branch is up to date with origin/main.\n\nnothing to commit, working tree clean\n"), len("On branch main (working tree clean)\n")), compacted: true, wantFamily: FamilyGitStatus,
		},
		{
			name: "dirty git status compacts", bin: "git",
			script: "printf 'On branch main\\nChanges not staged for commit:\\n\\tmodified:   a.go\\n'",
			argv:   []string{"git", "status"}, compact: true,
			wantCode: 0, wantClass: ExitClassZero, wantOut: "* main\n M a.go\n" + Marker(len("On branch main\nChanges not staged for commit:\n\tmodified:   a.go\n"), len("* main\n M a.go\n")), compacted: true, wantFamily: FamilyGitStatus,
		},
		{
			name: "git log is compacted to one line per commit", bin: "git",
			script: "printf 'commit 0123456789abcdef0123456789abcdef01234567\\nAuthor: A <a@example.com>\\n\\n    Init\\n'",
			argv:   []string{"git", "log"}, compact: true,
			wantCode: 0, wantClass: ExitClassZero, wantOut: "0123456 Init\n" + Marker(len("commit 0123456789abcdef0123456789abcdef01234567\nAuthor: A <a@example.com>\n\n    Init\n"), len("0123456 Init\n")), compacted: true, wantFamily: FamilyGitLog,
		},
		{
			name: "git log in an unknown shape stays raw", bin: "git",
			script: "printf 'commit 0123456789abcdef0123456789abcdef01234567\\ngpg: Signature made\\nAuthor: A <a@example.com>\\n\\n    Init\\n'",
			argv:   []string{"git", "log"}, compact: true,
			wantCode: 0, wantClass: ExitClassZero, wantOut: "commit 0123456789abcdef0123456789abcdef01234567\ngpg: Signature made\nAuthor: A <a@example.com>\n\n    Init\n", wantFamily: FamilyGitLog,
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
			name: "a failure that only wrote stderr keeps its code and its text", bin: "git",
			script: "echo 'fatal: not a git repository' >&2; exit 128",
			argv:   []string{"git", "status"}, compact: true,
			wantCode: 128, wantClass: ExitClassNonZero, wantErr: "fatal: not a git repository\n", wantFamily: FamilyGitStatus,
		},
		{
			name: "an exit code above 125 is the program's own", bin: "go",
			script: "printf 'partial output\\n'; exit 200",
			argv:   []string{"go", "test", "./..."}, compact: true,
			wantCode: 200, wantClass: ExitClassNonZero, wantOut: "partial output\n", wantFamily: FamilyGoTest,
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
