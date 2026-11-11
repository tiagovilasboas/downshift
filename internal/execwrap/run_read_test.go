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
	"testing"
)

// readSource writes a synthetic Go file of n funcs (11 lines each) under dir.
func readSource(t *testing.T, dir, name string, funcs int) (path, content string) {
	t.Helper()
	var b strings.Builder
	b.WriteString("package demo\n\nimport \"strings\"\n\n")
	for i := 0; i < funcs; i++ {
		fmt.Fprintf(&b, "func Step%d(s string) string {\n", i)
		for j := 0; j < 8; j++ {
			fmt.Fprintf(&b, "\ts = strings.TrimSpace(s) + \"%d\"\n", j)
		}
		b.WriteString("\treturn s\n}\n")
	}
	path = filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(b.String()), 0o600); err != nil {
		t.Fatal(err)
	}
	return path, b.String()
}

func TestRun_FileReadFoldsLongSourceAndKeepsShortOnes(t *testing.T) {
	dir := t.TempDir()
	long, raw := readSource(t, dir, "long.go", 40)
	short, shortRaw := readSource(t, dir, "short.go", 5)

	r, out, _ := run(t, []string{"cat", long}, true)
	if !r.Compacted || r.Family != FamilyFileRead || len(out) >= len(raw) {
		t.Fatalf("long file not folded: compacted=%v family=%q %d>=%d", r.Compacted, r.Family, len(out), len(raw))
	}
	for _, want := range []string{"5: func Step0(", "434: func Step39(", "full text: sed -n '1,444p' " + long, "[downshift compact: compacted"} {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks %q:\n%s", want, out)
		}
	}

	r, out, _ = run(t, []string{"cat", short}, true)
	if r.Compacted || out != shortRaw {
		t.Errorf("short file changed: compacted=%v", r.Compacted)
	}

	// sed -n: the view numbers lines from the range start.
	r, out, _ = run(t, []string{"sed", "-n", "101,444p", long}, true)
	if !r.Compacted || !strings.Contains(out, "101: ") || !strings.Contains(out, fmt.Sprintf("full text: sed -n '101,444p' %s]", long)) {
		t.Errorf("sed range view wrong (compacted=%v):\n%.300s", r.Compacted, out)
	}

	// head: the whole file requested is the file.
	r, out, _ = run(t, []string{"head", "-n", "400", long}, true)
	if !r.Compacted || !strings.Contains(out, "full text: sed -n '1,400p' "+long+"]") {
		t.Errorf("head view wrong (compacted=%v):\n%.300s", r.Compacted, out)
	}

	// tail: the numbers are relative and the hint says so.
	r, out, _ = run(t, []string{"tail", "-n", "400", long}, true)
	if !r.Compacted || !strings.Contains(out, "full text: tail -n 400 "+long+" (numbers count from the first line shown)]") {
		t.Errorf("tail view wrong (compacted=%v):\n%.300s", r.Compacted, out)
	}
}

func TestRun_FileReadNeverCompactsOrStoresSecretOrIrregularFiles(t *testing.T) {
	dir := t.TempDir()
	secret, raw := readSource(t, dir, "secrets.go", 40)
	envDir := filepath.Join(dir, ".aws")
	if err := os.Mkdir(envDir, 0o700); err != nil {
		t.Fatal(err)
	}
	inSecretDir, _ := readSource(t, envDir, "main.go", 40)

	for _, path := range []string{secret, inSecretDir} {
		var out, errb bytes.Buffer
		kept := false
		r := Run([]string{"cat", path}, Options{Stdin: strings.NewReader(""), Stdout: &out, Stderr: &errb, Compact: true,
			Keep: func([]byte) (string, bool) { kept = true; return "id", true }})
		if r.Compacted || kept || out.String() != raw {
			t.Errorf("%s: compacted=%v kept=%v, want the raw text and no CCR store", filepath.Base(path), r.Compacted, kept)
		}
	}

	// A directory is not a regular file: cat reports its own error, untouched.
	var out, errb bytes.Buffer
	r := Run([]string{"cat", dir}, Options{Stdin: strings.NewReader(""), Stdout: &out, Stderr: &errb, Compact: true})
	if r.Compacted || r.ExitCode == 0 || !strings.Contains(errb.String(), "directory") {
		t.Errorf("cat of a directory: compacted=%v exit=%d stderr=%q", r.Compacted, r.ExitCode, errb.String())
	}
}

// The kept-output hint names the session when the run has one, and is the plain
// `retrieve <id>` otherwise. KeptID stays the bare CCR id.
func TestRun_KeptHintCarriesSession(t *testing.T) {
	long, _ := readSource(t, t.TempDir(), "long.go", 40)
	for _, tc := range []struct{ session, want string }{
		{"", "full output: downshift retrieve abc123abc123]"},
		{"not-a-hash", "full output: downshift retrieve abc123abc123]"},
		{hashS2, "full output: downshift retrieve --session=aaaaaaaaaaaa abc123abc123]"},
	} {
		var out, errb bytes.Buffer
		r := Run([]string{"cat", long}, Options{Stdin: strings.NewReader(""), Stdout: &out, Stderr: &errb, Compact: true,
			Session: tc.session, Keep: func([]byte) (string, bool) { return "abc123abc123", true }})
		if !r.Compacted || !strings.Contains(out.String(), tc.want) || r.KeptID != "abc123abc123" {
			t.Errorf("session %q: compacted=%v kept=%q, want hint %q\n%.200s", tc.session, r.Compacted, r.KeptID, tc.want, out.String())
		}
	}
}

func TestRun_FileReadStoresRawThroughKeepOnlyWhenFolded(t *testing.T) {
	dir := t.TempDir()
	long, raw := readSource(t, dir, "long.go", 40)
	var stored []byte
	var out, errb bytes.Buffer
	r := Run([]string{"cat", long}, Options{Stdin: strings.NewReader(""), Stdout: &out, Stderr: &errb, Compact: true,
		Keep: func(b []byte) (string, bool) { stored = b; return "abc123abc123", true }})
	if !r.Compacted || string(stored) != raw || !strings.Contains(out.String(), "full output: downshift retrieve abc123abc123") {
		t.Errorf("compacted=%v stored=%d bytes (want %d)\n%.200s", r.Compacted, len(stored), len(raw), out.String())
	}

	// A short file with a run of same-shape lines is not folded through CCR.
	short := filepath.Join(dir, "table.go")
	table := strings.Repeat("\tcase 1: return 1\n", 20)
	if err := os.WriteFile(short, []byte(table), 0o600); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	called := false
	r = Run([]string{"cat", short}, Options{Stdin: strings.NewReader(""), Stdout: &out, Stderr: &errb, Compact: true,
		Keep: func([]byte) (string, bool) { called = true; return "id", true }})
	if r.Compacted || called || out.String() != table {
		t.Errorf("short file: compacted=%v kept=%v", r.Compacted, called)
	}
}

func TestRun_WcPassesThrough(t *testing.T) {
	dir := t.TempDir()
	path, _ := readSource(t, dir, "a.go", 3)
	r, out, _ := run(t, []string{"wc", "-l", path}, true)
	if r.Family != FamilyWc || r.Compacted || !strings.Contains(out, path) {
		t.Errorf("wc: family=%q compacted=%v out=%q", r.Family, r.Compacted, out)
	}
}

func TestRun_SearchBreadthForms(t *testing.T) {
	rows := strings.Repeat("internal/some/long/path/module.go:\treturn handleRequest(ctx, req) // see: handler\n", 12)
	list := "internal/some/long/path/module.go\n"
	for _, n := range []string{"other", "third", "fourth", "fifth", "sixth"} {
		list += "internal/some/long/path/" + n + ".go\n"
	}
	counts := "internal/a/one.go:3\n" + strings.Repeat("internal/a/zero.go:0\n", 9)
	cases := []struct {
		name, bin string
		argv      []string
		raw       string
		want      []string
	}{
		{"rg without -n", "rg", []string{"rg", "handleRequest", "internal"}, rows, []string{"internal/some/long/path/module.go\n return handleRequest("}},
		{"grep -r without -n", "grep", []string{"grep", "-r", "handleRequest", "internal"}, rows, []string{"internal/some/long/path/module.go\n return handleRequest("}},
		{"rg -l", "rg", []string{"rg", "-l", "handleRequest", "internal"}, list, []string{"module.go\n"}},
		{"grep -rl", "grep", []string{"grep", "-rl", "handleRequest", "internal"}, list, []string{"module.go\n"}},
		{"rg -c", "rg", []string{"rg", "-c", "handleRequest", "internal"}, counts, []string{"... 9 files with 0 matches folded"}},
		{"grep -rc", "grep", []string{"grep", "-rc", "handleRequest", "internal"}, counts, []string{"... 9 files with 0 matches folded"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fakeBin(t, tc.bin, "cat <<'EOF'\n"+tc.raw+"EOF")
			r, out, _ := run(t, tc.argv, true)
			if r.ExitCode != 0 || !r.Compacted || len(out) >= len(tc.raw) {
				t.Fatalf("compacted=%v %d>=%d\n%s", r.Compacted, len(out), len(tc.raw), out)
			}
			for _, w := range tc.want {
				if !strings.Contains(out, w) {
					t.Errorf("output lacks %q:\n%s", w, out)
				}
			}
		})
	}
}

// One file operand: grep and rg print rows with no path, so a row is never read
// as path:text and the output is byte-identical.
func TestRun_SearchRowsWithoutPathStayRaw(t *testing.T) {
	raw := strings.Repeat("note: handleRequest is called here\n", 12)
	fakeBin(t, "rg", "cat <<'EOF'\n"+raw+"EOF")
	r, out, _ := run(t, []string{"rg", "handleRequest", "notes.txt"}, true)
	if r.Compacted || out != raw {
		t.Errorf("compacted=%v, want raw", r.Compacted)
	}
}
