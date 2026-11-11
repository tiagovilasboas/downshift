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
	if !strings.Contains(got, "internal/pkg0/file.go:25:"+strings.Repeat("x", 80-len("internal/pkg0/file.go:25:"))+" … (") {
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
