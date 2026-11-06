// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import (
	"fmt"
	"strings"
	"testing"
)

func TestCompressGrepRows(t *testing.T) {
	roots := CommandHint{Roots: []string{"src", "./pkg", "."}}
	cases := []struct {
		name    string
		in      string
		hint    CommandHint
		applied bool
		want    string
	}{
		{
			name: "rows grouped under their path, leading space trimmed",
			in: "src/a/long_file_name.go:\treturn ok // see: x\nsrc/a/long_file_name.go:    return no\n" +
				"src/b/other_file_name.go:var x = 1\n",
			hint: roots, applied: true,
			want: "src/a/long_file_name.go\n return ok // see: x\n return no\nsrc/b/other_file_name.go\n var x = 1\n",
		},
		{"dot root accepts ./ paths", "./a/long_file_name.go:one\n./a/long_file_name.go:two\n./a/long_file_name.go:three\n", roots, true,
			"./a/long_file_name.go\n one\n two\n three\n"},
		{"one file operand prints bare text", "note: item 1\nnote: item 2\n", CommandHint{Roots: []string{"notes.txt"}}, false, ""},
		{"row under no operand", "other/a.go:x\nother/a.go:y\n", CommandHint{Roots: []string{"src"}}, false, ""},
		{"sibling directory is not under the root", "srcs/a.go:x\nsrcs/a.go:y\n", CommandHint{Roots: []string{"src"}}, false, ""},
		{"no roots", "src/a.go:x\nsrc/a.go:y\n", CommandHint{}, false, ""},
		{"row without a colon", "src/a.go:x\nbinary file matches\n", roots, false, ""},
	}
	for _, tc := range cases {
		out, ok, _ := compressGrepRows([]byte(tc.in), tc.hint)
		if ok != tc.applied || (ok && string(out) != tc.want) || (!ok && string(out) != tc.in) {
			t.Errorf("%s: ok=%v out=%q", tc.name, ok, out)
		}
	}
}

func TestCompressGrepCaps(t *testing.T) {
	var b strings.Builder
	for i := 0; i < 30; i++ { // one file past the per-file cap
		fmt.Fprintf(&b, "internal/wide/one.go:%d:match number %d\n", i+1, i)
	}
	for f := 0; f < 15; f++ { // 15 files of 15 rows: past the total cap
		for i := 0; i < 15; i++ {
			fmt.Fprintf(&b, "internal/wide/f%d.go:%d:match number %d\n", f, i+1, i)
		}
	}
	out, ok, _ := compressGrep([]byte(b.String()))
	text := string(out)
	if !ok {
		t.Fatal("not applied")
	}
	if !strings.Contains(text, " ... 10 more in this file\n") {
		t.Errorf("per-file cut not counted:\n%s", text)
	}
	if n := strings.Count(text, "match number"); n != grepTotalCap {
		t.Errorf("kept %d rows, want the total cap %d", n, grepTotalCap)
	}
	// 20 kept of one.go, then 12 files of 15 rows fill the 200; 3 files (45 rows) are left out whole.
	if !strings.Contains(text, "... 45 more matches in 3 more files") {
		t.Errorf("total cut not counted:\n%s", text)
	}
}

func TestCompressGrepWithinCapsIsLossless(t *testing.T) {
	in := "a/b/c.go:1:one\na/b/c.go:2:two\na/b/d.go:9:nine\n"
	out, ok, _ := compressGrep([]byte(in))
	if !ok || strings.Contains(string(out), "more") {
		t.Errorf("small output cut: ok=%v %q", ok, out)
	}
}

func TestCompressGrepCounts(t *testing.T) {
	zeros := strings.Repeat("src/x/empty.go:0\n", 10)
	in := "src/a.go:3\n" + zeros + "src/b.go:12\n"
	out, ok, _ := compressGrepCounts([]byte(in))
	if !ok || string(out) != "src/a.go:3\nsrc/b.go:12\n... 10 files with 0 matches folded\n" {
		t.Errorf("counts: ok=%v %q", ok, out)
	}
	for _, raw := range []string{"42\n", "src/a.go:3\nbinary file\n", "src/a.go:x\nsrc/b.go:1\n"} {
		if out, ok, _ := compressGrepCounts([]byte(raw)); ok || string(out) != raw {
			t.Errorf("%q should stay raw, got %q", raw, out)
		}
	}
	var big strings.Builder
	for i := 0; i < grepTotalCap+5; i++ {
		fmt.Fprintf(&big, "src/dir%d/file_with_a_long_name.go:2\n", i)
	}
	out, ok, _ = compressGrepCounts([]byte(big.String()))
	if !ok || !strings.Contains(string(out), "... 5 more files with matches (10 matches) cut") {
		t.Errorf("count cap not counted: ok=%v tail=%q", ok, out[len(out)-120:])
	}
}
