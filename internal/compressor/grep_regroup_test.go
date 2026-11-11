// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import (
	"strings"
	"testing"
)

// unregroup rebuilds the original rows from RegroupMatches output: a line
// that does not begin with a space is a path, every other line is a row of it.
func unregroup(t *testing.T, out string) string {
	t.Helper()
	var b strings.Builder
	path := ""
	for _, l := range strings.Split(strings.TrimSuffix(out, "\n"), "\n") {
		if !strings.HasPrefix(l, " ") {
			path = l
			continue
		}
		if path == "" {
			t.Fatalf("row before any path: %q", l)
		}
		b.WriteString(path + ":" + l[1:] + "\n")
	}
	return b.String()
}

func TestRegroupMatches_Lossless(t *testing.T) {
	cases := map[string]string{
		"one file": strings.Repeat("internal/execwrap/compound_search.go:12:\tif keep == nil {\n", 6),
		"two files, leading space and colons kept": "internal/core/route.go:1:  a: b:c\n" +
			"internal/core/route.go:20:\t\treturn d // x:y\n" +
			"internal/core/route.go:31:\n" +
			"internal/core/observe.go:4:package core\n" +
			"internal/core/observe.go:9:func ObserveSpawn(req Request) Decision {\n",
		"a path seen again after another": "internal/a/one.go:1:x\ninternal/a/one.go:2:y\ninternal/a/one.go:3:z\n" +
			"internal/b/two.go:1:x\ninternal/b/two.go:2:y\ninternal/a/one.go:9:w\ninternal/a/one.go:10:v\n",
		"CRLF rows":        "internal/c/crlf.go:1:a\r\ninternal/c/crlf.go:2:b\r\ninternal/c/crlf.go:3:c\r\n",
		"no final newline": "internal/d/last.go:1:a\ninternal/d/last.go:2:b\ninternal/d/last.go:3:c",
	}
	for name, in := range cases {
		out, ok := RegroupMatches([]byte(in))
		if !ok {
			t.Errorf("%s: not regrouped", name)
			continue
		}
		want := strings.TrimSuffix(in, "\n") + "\n"
		if got := unregroup(t, string(out)); got != want {
			t.Errorf("%s: lossy\n got %q\nwant %q", name, got, want)
		}
		if len(out)*4 > len(in)*3 {
			t.Errorf("%s: %d to %d bytes, under a quarter", name, len(in), len(out))
		}
	}
}

func TestRegroupMatches_KeepsRaw(t *testing.T) {
	rows := strings.Repeat("internal/execwrap/compound_search.go:12:x\n", 5)
	cases := map[string]string{
		"empty":            "",
		"separator":        rows + "--\n" + rows,
		"context row":      rows + "internal/execwrap/compound_search.go-13-y\n",
		"binary file":      rows + "Binary file internal/x.bin matches\n",
		"blank line":       rows + "\n" + rows,
		"no line number":   strings.Repeat("internal/execwrap/compound_search.go:x\n", 5),
		"path with space":  strings.Repeat(" internal/a.go:1:x\n", 5),
		"file text":        "package main\n\nfunc main() {}\n",
		"one row per file": "a.go:1:x\nb.go:2:y\nc.go:3:z\n",
	}
	for name, in := range cases {
		if out, ok := RegroupMatches([]byte(in)); ok || string(out) != in {
			t.Errorf("%s: regrouped to %q, want raw", name, out)
		}
	}
}
