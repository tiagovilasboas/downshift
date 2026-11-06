// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

func numbered(n int, prefix string) string {
	var b strings.Builder
	for i := 0; i < n; i++ {
		fmt.Fprintf(&b, "%s %d\n", prefix, i)
	}
	return b.String()
}

func blob(n int) string {
	const alpha = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"
	var b strings.Builder
	for i := 0; i < n; i++ {
		b.WriteByte(alpha[(i*7+i/13)%len(alpha)])
	}
	return b.String()
}

func TestElideBase64(t *testing.T) {
	cases := []struct {
		name, in, want string
		n              int
	}{
		{"data uri png", "shot: data:image/png;base64," + blob(51200) + " done", "shot: [base64 image/png, 38 KB elided] done", 1},
		{"bare blob", "x " + blob(2048) + "\n", "x [base64, 2 KB elided]\n", 1},
		{"short blob kept", "x " + blob(1000), "x " + blob(1000), 0},
		{"single class kept", strings.Repeat("+", 1100), strings.Repeat("+", 1100), 0},
		{"hex digest kept", strings.Repeat("ab12", 400), strings.Repeat("ab12", 400), 0},
		{"two blobs", blob(1100) + "\n" + blob(1100), "[base64, 1 KB elided]\n[base64, 1 KB elided]", 2},
	}
	for _, c := range cases {
		got, n := elideBase64(c.in)
		if got != c.want || n != c.n {
			t.Errorf("%s: got %q (%d), want %q (%d)", c.name, got, n, c.want, c.n)
		}
	}
}

func TestDedupLines(t *testing.T) {
	cases := []struct {
		name   string
		in     []string
		want   []string
		folded bool
	}{
		{"three fold", []string{"a", "a", "a", "b"}, []string{"a [x3]", "b"}, true},
		{"two stay", []string{"a", "a", "b"}, []string{"a", "a", "b"}, false},
		{"blank run stays", []string{"", "", "", ""}, []string{"", "", "", ""}, false},
		{"non adjacent stay", []string{"a", "b", "a", "b", "a"}, []string{"a", "b", "a", "b", "a"}, false},
		{"five", []string{"x", "x", "x", "x", "x"}, []string{"x [x5]"}, true},
	}
	for _, c := range cases {
		got, folded := dedupLines(c.in)
		if strings.Join(got, "|") != strings.Join(c.want, "|") || folded != c.folded {
			t.Errorf("%s: got %q (%v), want %q (%v)", c.name, got, folded, c.want, c.folded)
		}
	}
}

func TestCapLines(t *testing.T) {
	mk := func(n int) []string { return strings.Split(strings.TrimSuffix(numbered(n, "l"), "\n"), "\n") }
	if got, capped := capLines(mk(252)); capped || len(got) != 252 {
		t.Fatalf("252 lines must stay: capped=%v len=%d", capped, len(got))
	}
	got, capped := capLines(mk(10000))
	if !capped || len(got) != 251 {
		t.Fatalf("10000 lines: capped=%v len=%d, want 251", capped, len(got))
	}
	if got[0] != "l 0" || got[49] != "l 49" || got[50] != "[9750 lines elided]" || got[51] != "l 9800" || got[250] != "l 9999" {
		t.Fatalf("head/marker/tail wrong: %q %q %q %q %q", got[0], got[49], got[50], got[51], got[250])
	}
}

// A long log whose middle holds skipped, missing and vulnerable lines: the cut
// must not drop them, and the markers must still account for every cut line.
func TestCapLinesKeepsFlaggedLines(t *testing.T) {
	lines := strings.Split(strings.TrimSuffix(numbered(1000, "processing item"), "\n"), "\n")
	want := []string{"SKIPPED integration suite (no docker)", "3 packages missing from lockfile", "found 2 vulnerabilities"}
	lines[300], lines[301], lines[700] = want[0], want[1], want[2]
	got, capped := capLines(lines)
	if !capped {
		t.Fatal("expected a cut")
	}
	var idx []int
	elided := 0
	for i, l := range got {
		for _, w := range want {
			if l == w {
				idx = append(idx, i)
			}
		}
		var n int
		if _, err := fmt.Sscanf(l, "[%d lines elided]", &n); err == nil {
			elided += n
		}
	}
	if len(idx) != 3 || !(idx[0] < idx[1] && idx[1] < idx[2]) {
		t.Fatalf("flagged lines lost or out of order: %v", idx)
	}
	if elided != 1000-capHead-capTail-3 {
		t.Fatalf("markers account for %d lines, want %d", elided, 1000-capHead-capTail-3)
	}
}

func TestCompressGeneric(t *testing.T) {
	png := "data:image/png;base64," + blob(60000)
	cases := []struct {
		name     string
		in       string
		applied  bool
		contains string
	}{
		{"playwright base64 screenshot", "step 1\n" + png + "\nstep 2\n", true, "[base64 image/png,"},
		{"10k lines capped", numbered(10000, "processing item"), true, "[9750 lines elided]"},
		{"repeated lines fold", "start\n" + strings.Repeat("tick\n", 40) + "end\n", true, "tick [x40]"},
		{"nothing to cut", "hello\nworld\n", false, ""},
		{"error keeps raw", numbered(500, "item") + "error: boom\n", false, ""},
		{"warning keeps raw", strings.Repeat("tick\n", 40) + "warning: x\n", false, ""},
		{"failed keeps raw", strings.Repeat("tick\n", 40) + "1 test failed\n", false, ""},
		{"stack frame keeps raw", strings.Repeat("tick\n", 40) + "    at foo (/a/b.js:3:4)\n", false, ""},
		{"cross mark keeps raw", strings.Repeat("tick\n", 40) + "✗ check\n", false, ""},
		{"denied keeps raw", strings.Repeat("tick\n", 40) + "Permission denied\n", false, ""},
		{"npm warn keeps raw", numbered(400, "line") + "npm WARN deprecated foo\n", false, ""},
		{"npm err keeps raw", "a\n" + strings.Repeat("npm ERR! code E404\n", 4), false, ""},
		{"shell not found keeps raw", numbered(400, "line") + "bash: x: command not found\n", false, ""},
		{"error hidden in run keeps raw", "start\n" + strings.Repeat("Error", 250) + "\nq\nq\nq\n", false, ""},
		{"existing marker keeps raw", strings.Repeat("tick [x3]\n", 40), false, ""},
		{"danger beside blob keeps raw", png + "\nFATAL\n", false, ""},
	}
	for _, c := range cases {
		out, applied, _ := compressGeneric([]byte(c.in))
		if applied != c.applied {
			t.Errorf("%s: applied=%v, want %v", c.name, applied, c.applied)
			continue
		}
		if !applied {
			if string(out) != c.in {
				t.Errorf("%s: output not byte for byte", c.name)
			}
			continue
		}
		if !strings.Contains(string(out), c.contains) || len(out) >= len(c.in) {
			t.Errorf("%s: missing %q or no gain", c.name, c.contains)
		}
	}
}

func TestCompressGenericReduction(t *testing.T) {
	in := "run\n" + "data:image/png;base64," + blob(200000) + "\ndone\n"
	out, ok, _ := compressGeneric([]byte(in))
	if !ok || float64(len(out))/float64(len(in)) > 0.01 {
		t.Fatalf("base64 screenshot must shrink by 99%%: %d of %d", len(out), len(in))
	}
}

func TestCompressExitGenericFormat(t *testing.T) {
	in := []byte(numbered(10000, "processing item"))
	res := CompressExit(in, ModeSafe, 0)
	if !res.Applied || res.Format != FormatGeneric {
		t.Fatalf("applied=%v format=%s reason=%s", res.Applied, res.Format, res.Reason)
	}
	if got := CompressExit(in, ModeSafe, 1); got.Applied || string(got.Output) != string(in) {
		t.Fatal("non-zero exit must stay raw")
	}
	if got := CompressExit(in, ModeSafe, -1); got.Applied {
		t.Fatal("unknown exit must stay raw for the generic chain")
	}
	if got := CompressExit([]byte("hello\n"), ModeSafe, 0); got.Format != FormatUnknown {
		t.Fatalf("untouched text keeps format unknown, got %s", got.Format)
	}
}

// The bound is loose on purpose: the race detector runs this much slower, and
// the test only has to catch a quadratic scan.
func TestElideBase64LinearOnHugeLine(t *testing.T) {
	line := strings.Repeat("x", 2*1024*1024-1) + "\n"
	start := time.Now()
	compressGeneric([]byte(line))
	if d := time.Since(start); d > 10*time.Second {
		t.Fatalf("2 MB line took %v", d)
	}
}
