// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import "testing"

func TestStripNoise(t *testing.T) {
	const unix = "/Users/x/repo"
	cases := []struct {
		name   string
		cwd    string
		format Format
		in     string
		want   string
	}{
		{"unix path with line", unix, FormatGrep,
			"/Users/x/repo/src/a.ts:3\n", "src/a.ts:3\n"},
		{"trailing slash cwd", unix + "/", FormatGrep,
			"/Users/x/repo/src/a.ts\n", "src/a.ts\n"},
		{"several per line", unix, FormatFind,
			"/Users/x/repo/a /Users/x/repo/b\n", "a b\n"},
		{"sibling directory kept", unix, FormatGrep,
			"/Users/x/repo-old/a.ts\n", "/Users/x/repo-old/a.ts\n"},
		{"glued to another path kept", unix, FormatGrep,
			"/mnt/Users/x/repo/a.ts\n", "/mnt/Users/x/repo/a.ts\n"},
		{"bare directory kept", unix, FormatFind,
			"/Users/x/repo/\n", "/Users/x/repo/\n"},
		{"danger line raw", unix, FormatGrep,
			"/Users/x/repo/a.ts:1: error: bad\n/Users/x/repo/b.ts:2\n",
			"/Users/x/repo/a.ts:1: error: bad\nb.ts:2\n"},
		{"secret line raw", unix, FormatGrep,
			"/Users/x/repo/.env:1: API_KEY=1\n", "/Users/x/repo/.env:1: API_KEY=1\n"},
		{"git diff untouched", unix, FormatGitDiff,
			"+/Users/x/repo/a\n", "+/Users/x/repo/a\n"},
		{"empty cwd", "", FormatGrep, "/Users/x/repo/a\n", "/Users/x/repo/a\n"},
		{"root cwd", "/", FormatGrep, "/etc/a\n", "/etc/a\n"},
		{"relative cwd", "repo", FormatGrep, "repo/a\n", "repo/a\n"},
		{"windows backslash", `C:\Users\x\repo`, FormatGrep,
			`C:\Users\x\repo\src\a.ts:3` + "\n", `src\a.ts:3` + "\n"},
		{"windows forward slash", `C:\Users\x\repo`, FormatGrep,
			"C:/Users/x/repo/src/a.ts:3\n", "src/a.ts:3\n"},
		{"windows drive root", `C:\`, FormatGrep, `C:\a` + "\n", `C:\a` + "\n"},
		{"windows sibling kept", `C:\Users\x\repo`, FormatGrep,
			`C:\Users\x\repo2\a` + "\n", `C:\Users\x\repo2\a` + "\n"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			in := Result{Output: []byte(c.in), Applied: true, Format: c.format,
				OriginalBytes: 1000, ReducedBytes: len(c.in)}
			got := StripNoise(in, c.cwd)
			if string(got.Output) != c.want {
				t.Fatalf("output = %q, want %q", got.Output, c.want)
			}
			if got.ReducedBytes != len(got.Output) {
				t.Fatalf("ReducedBytes = %d, want %d", got.ReducedBytes, len(got.Output))
			}
		})
	}
}

func TestStripNoiseLeavesRawResultAlone(t *testing.T) {
	in := Result{Output: []byte("/Users/x/repo/a\n"), Applied: false, Format: FormatGrep}
	if got := StripNoise(in, "/Users/x/repo"); string(got.Output) != string(in.Output) {
		t.Fatalf("raw result changed: %q", got.Output)
	}
}

func TestStripNoiseKeepsDurations(t *testing.T) {
	const s = "Vitest: PASS (25 passed across 5 files in 412ms)\n"
	in := Result{Output: []byte(s), Applied: true, Format: FormatVitest}
	if got := StripNoise(in, "/Users/x/repo"); string(got.Output) != s {
		t.Fatalf("summary changed: %q", got.Output)
	}
}
