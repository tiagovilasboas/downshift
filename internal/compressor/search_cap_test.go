// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import (
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"
)

// searchRows prints n rows for each file, rows of width bytes of text.
func searchRows(files, n, width int) string {
	var b strings.Builder
	for f := 0; f < files; f++ {
		for i := 1; i <= n; i++ {
			fmt.Fprintf(&b, "internal/pkg%d/file.go:%d:%s\n", f, i, strings.Repeat("x", width))
		}
	}
	return b.String()
}

func TestCapSearchRows_Keeps(t *testing.T) {
	cases := map[string]string{
		"empty":                   "",
		"few rows":                searchRows(3, 5, 20),
		"25 rows in one file":     searchRows(1, 25, 200),
		"200 rows over 10 files":  searchRows(10, 20, 20),
		"long rows, nothing cut":  searchRows(2, 10, 400),
		"one file, 26 short rows": searchRows(1, 26, 1), // the count line costs what the dropped row saved
	}
	for name, in := range cases {
		if out, ok := CapSearchRows([]byte(in)); ok || string(out) != in {
			t.Errorf("%s: cut to\n%s", name, out)
		}
	}
}

func TestCapSearchRows_PerFile(t *testing.T) {
	in := searchRows(2, 30, 120)
	out, ok := CapSearchRows([]byte(in))
	if !ok {
		t.Fatal("not cut")
	}
	got := string(out)
	for f := 0; f < 2; f++ {
		p := fmt.Sprintf("internal/pkg%d/file.go:", f)
		if n := strings.Count(got, p); n != 25 {
			t.Errorf("file %d: %d rows kept, want 25", f, n)
		}
		if !strings.Contains(got, fmt.Sprintf("  … 5 more in internal/pkg%d/file.go\n", f)) {
			t.Errorf("file %d: no count line:\n%s", f, got)
		}
	}
	if !strings.Contains(got, "internal/pkg0/file.go:25:"+strings.Repeat("x", 80-len("internal/pkg0/file.go:25:"))+" …\n") {
		t.Errorf("kept row not cut at 80 bytes:\n%s", got)
	}
	if strings.Contains(got, "internal/pkg0/file.go:26:") {
		t.Errorf("row 26 kept:\n%s", got)
	}
}

func TestCapSearchRows_TotalAndOrder(t *testing.T) {
	in := searchRows(12, 20, 10) // 242 rows with the two below: 200 shown, so file 9 loses 2 and files 10, 11 are left out
	in = "internal/late/a.go:1:x\n" + in + "internal/late/a.go:2:y\n"
	out, ok := CapSearchRows([]byte(in))
	if !ok {
		t.Fatal("not cut")
	}
	got := string(out)
	if !strings.HasPrefix(got, "internal/late/a.go:1:x\ninternal/late/a.go:2:y\ninternal/pkg0/file.go:1:") {
		t.Errorf("rows not grouped by file in first-seen order:\n%.200s", got)
	}
	if !strings.HasSuffix(got, "internal/pkg9/file.go:18:xxxxxxxxxx\n  … 2 more in internal/pkg9/file.go\n… 40 more matches in 2 more files\n") {
		t.Errorf("tail count wrong:\n%s", got[len(got)-200:])
	}
}

func TestCapSearchRows_CutsOnRuneBoundary(t *testing.T) {
	row := "a.go:1:" + strings.Repeat("é", 60) + "\n"
	in := strings.Repeat(row, 30)
	out, ok := CapSearchRows([]byte(in))
	if !ok {
		t.Fatal("not cut")
	}
	for _, l := range strings.Split(string(out), "\n") {
		if !utf8.ValidString(l) {
			t.Errorf("invalid UTF-8: %q", l)
		}
	}
}

// joinRows joins rows into search output.
func joinRows(rows ...string) string { return strings.Join(rows, "\n") + "\n" }

func TestCapSearchRows_NotSearchRows(t *testing.T) {
	var files, counted strings.Builder
	for i := 0; i < 100; i++ {
		fmt.Fprintf(&files, "internal/pkg%d/file.go\n", i)
		fmt.Fprintf(&counted, "%7d ident%d\n", 199-i, i)
	}
	for name, in := range map[string]string{"grep -l file list": files.String(), "uniq -c counted list": counted.String()} {
		if out, ok := CapSearchRows([]byte(in)); ok || string(out) != in {
			t.Errorf("%s: cut to\n%s", name, out)
		}
	}
}

func TestCapSearchRows_WindowsDrive(t *testing.T) {
	var b strings.Builder
	for f := 0; f < 60; f++ {
		for i := 1; i <= 2; i++ {
			fmt.Fprintf(&b, "C:\\repo\\f%d.go:%d:%s\n", f, i, strings.Repeat("y", 30))
		}
	}
	b.WriteString("D:/repo/big.go:1:a\n")
	for i := 2; i <= 30; i++ {
		fmt.Fprintf(&b, "D:/repo/big.go:%d:%s\n", i, strings.Repeat("y", 30))
	}
	out, ok := CapSearchRows([]byte(b.String()))
	if !ok {
		t.Fatal("not cut")
	}
	got := string(out)
	if !strings.Contains(got, "  … 5 more in D:/repo/big.go\n") {
		t.Errorf("no per-file count for the drive path:\n%s", got)
	}
	if strings.Contains(got, "more in C\n") || strings.Contains(got, "more in D\n") {
		t.Errorf("file key lost its path:\n%s", got)
	}
	if !strings.Contains(got, "C:\\repo\\f0.go:2:") {
		t.Errorf("rows of a short file were cut:\n%s", got)
	}
}

func TestCapSearchRows_Context(t *testing.T) {
	var rows []string
	for i := 1; i <= 30; i++ { // a match every other line, context between
		rows = append(rows, fmt.Sprintf("a.go-%d-ctx before", 2*i-1), fmt.Sprintf("a.go:%d:match %d", 2*i, i), "--")
	}
	rows = append(rows, "b.go-3-ctx before", "b.go:4:other", "b.go-5-ctx after")
	out, ok := CapSearchRows([]byte(joinRows(rows...)))
	if !ok {
		t.Fatal("not cut")
	}
	got := string(out)
	want := "a.go-1-ctx before\na.go:2:match 1\n--\na.go-3-ctx before\na.go:4:match 2\n--\n"
	if !strings.HasPrefix(got, want) {
		t.Errorf("context not adjacent and in order:\n%.200s", got)
	}
	if !strings.Contains(got, "a.go-25-ctx before\n  … 35 more in a.go\n") {
		t.Errorf("a.go not cut at 25 rows or separator kept before the count:\n%s", got)
	}
	if strings.Contains(got, "--\n  …") || strings.Contains(got, "--\n--") {
		t.Errorf("stray separator:\n%s", got)
	}
	if !strings.Contains(got, "b.go-3-ctx before\nb.go:4:other\nb.go-5-ctx after\n") {
		t.Errorf("second file regrouped:\n%s", got)
	}
}

func TestCapSearchRows_Mixed(t *testing.T) {
	// 10% or less of stray lines still cuts; they follow the row before them.
	in := searchRows(1, 40, 10) + "Binary file x matches\n"
	out, ok := CapSearchRows([]byte(in))
	if !ok || !strings.Contains(string(out), "  … 16 more in internal/pkg0/file.go\n") {
		t.Errorf("stray line stopped the cut or was lost:\n%s", out)
	}
	// Past that share it is not search output.
	in = searchRows(1, 40, 10) + strings.Repeat("plain line\n", 10)
	if out, ok := CapSearchRows([]byte(in)); ok || string(out) != in {
		t.Errorf("mostly unparsed output cut:\n%s", out)
	}
}

func TestCapSearchRows_ReviewShapes(t *testing.T) {
	var b strings.Builder
	for i := 1; i <= 300; i++ {
		fmt.Fprintf(&b, "a.go:%d:hello world line\n", i)
	}
	rows := b.String()
	if _, ok := CapSearchRows([]byte(rows + strings.Repeat("plain file text\n", 20))); ok {
		t.Error("search followed by file text was cut")
	}
	if _, ok := CapSearchRows([]byte(rows + "Binary file x.bin matches\n")); !ok {
		t.Error("one stray notice stopped the cut")
	}
	var c strings.Builder
	for i := 1; i <= 30; i++ {
		fmt.Fprintf(&c, "a.go-%d-  at 10:30:45 done\r\na.go:%d:match\r\n--\r\n", i*3, i*3+1)
	}
	out, ok := CapSearchRows([]byte(c.String()))
	if !ok {
		t.Fatal("CRLF context output not cut")
	}
	s := string(out)
	if !strings.Contains(s, "  … 35 more in a.go\n") || strings.Contains(s, "10\n") || strings.Count(s, "  … ") != 1 {
		t.Errorf("context with a timestamp or CRLF separator grouped wrong:\n%s", s)
	}
}
