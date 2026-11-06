// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import (
	"strings"
	"testing"
)

func TestCompressMarkdownView(t *testing.T) {
	issue := readCorpus(t, "gh_issue_view_clean.txt")
	res := Compress([]byte(issue), ModeSafe)
	if !res.Applied || res.Format != FormatMarkdown {
		t.Fatalf("applied=%v format=%s reason=%s", res.Applied, res.Format, res.Reason)
	}
	out := string(res.Output)
	for _, want := range []string{
		"title:\tPhase 3: Content Transformation\n", "number:\t812\n--\n",
		"[the design](https://example.com/design?a=1&b=2)", "[roadmap](docs/roadmap.md)",
		"| Task | Estimate |\n|------|----------|\n| 3.1  | ~60 LOC  |",
		"```go\nfunc main() {\n\n\n\trun()\n}\n```", "    indented code\n\n\n    stays as is",
		"## Goal\nStructure-aware",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in\n%s", want, out)
		}
	}
	for _, cut := range []string{"<!--", "img.shields.io", "## Goal\n## Goal", "\n\n\n\n", "Content Transformation\n\n"} {
		if strings.Contains(out, cut) {
			t.Errorf("kept %q in\n%s", cut, out)
		}
	}
	if strings.Count(out, "## Goal") != 1 {
		t.Errorf("heading not deduplicated:\n%s", out)
	}
}

func TestCompressMarkdownViewRelease(t *testing.T) {
	res := Compress([]byte(readCorpus(t, "gh_release_view_clean.txt")), ModeSafe)
	if !res.Applied || res.Format != FormatMarkdown {
		t.Fatalf("applied=%v format=%s reason=%s", res.Applied, res.Format, res.Reason)
	}
	want := "--\n## What's Changed\n\n* Add compaction by @x in https://github.com/o/r/pull/1\n\n**Full Changelog**: https://github.com/o/r/compare/v0.5.0...v0.6.0\n"
	if !strings.HasSuffix(string(res.Output), want) {
		t.Fatalf("got\n%s", res.Output)
	}
}

func TestCompressMarkdownViewKeepsRaw(t *testing.T) {
	hdr := "title:\tT\nstate:\tOPEN\ncomments:\t0\nnumber:\t1\n--\n"
	cases := map[string]string{
		"error word in prose": hdr + "An error occurs\n\n\nmore\n",
		"warning word":        hdr + "Warning: x\n\n\nmore\n",
		"stack frame":         hdr + "see\n\n\n    at foo (/a/b.js:1:2)\n",
		"python traceback":    hdr + "Traceback (most recent call last)\n\n\n",
		"open fence":          hdr + "```\ncode\n\n\n",
		"unknown header line": "title:\tT\nstate:\tOPEN\ncomments:\t0\nnumber:\t1\nstray line\n--\nx\n\n\ny\n",
		"nothing to remove":   hdr + "a\n\nb\n",
		"badge with text":     hdr + "[![CI](https://img.shields.io/x)](u) status\nb\n",
	}
	for name, in := range cases {
		t.Run(name, func(t *testing.T) {
			res := Compress([]byte(in), ModeSafe)
			if string(res.Output) != in {
				t.Fatalf("not raw (format=%s reason=%s):\n%q", res.Format, res.Reason, res.Output)
			}
		})
	}
}
