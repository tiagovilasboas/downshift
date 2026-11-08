// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor_test

import (
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift-ai/internal/compressor"
)

// danglingDiff builds a one-hunk diff: nine lines, line five changed. ctxFirst
// replaces the first context line (which trimming drops), minus and plus
// replace the removed and added line, and tail is appended after the diff.
func danglingDiff(ctxFirst, minus, plus, tail string) string {
	return "diff --git a/h.go b/h.go\nindex abc123..def456 100644\n--- a/h.go\n+++ b/h.go\n" +
		"@@ -1,9 +1,9 @@ func handle() {\n" +
		" " + ctxFirst + "\n c2\n c3\n c4\n-" + minus + "\n+" + plus + "\n c6\n c7\n c8\n c9\n" + tail
}

func TestGitDiffDangerGate(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		applied bool
		reason  string
	}{
		{"context-only danger word compacts", danglingDiff("// handle the error case", "old()", "new()", ""), true, ""},
		{"context-only fail and panic compact", danglingDiff("panic(\"fail\")", "old()", "new()", ""), true, ""},
		{"removed line danger raw", danglingDiff("c1", "return failed()", "new()", ""), false, "git_diff_danger_word_preserved"},
		{"added line danger raw", danglingDiff("c1", "old()", "return fmt.Errorf(\"x\")", ""), false, "git_diff_danger_word_preserved"},
		{"stray error line raw", danglingDiff("c1", "old()", "new()", "error: something broke\n"), false, "git_diff_danger_word_preserved"},
		{"stray warning line raw", danglingDiff("c1", "old()", "new()", "warning: LF will be replaced\n"), false, "git_diff_danger_word_preserved"},
		{"stray indented line after hunk raw", danglingDiff("c1", "old()", "new()", "  warning: indented\n"), false, "git_diff_danger_word_preserved"},
		{"hunk header danger raw", strings.Replace(danglingDiff("c1", "old()", "new()", ""), "func handle()", "func onError()", 1), false, "git_diff_danger_word_preserved"},
		{"file header danger raw", strings.ReplaceAll(danglingDiff("c1", "old()", "new()", ""), "h.go", "fail.go"), false, "git_diff_danger_word_preserved"},
		{"secret on context line raw", danglingDiff("password := \"hunter2\"", "old()", "new()", ""), false, "git_diff_secret_preserved"},
		{"secret on change line raw", danglingDiff("c1", "old()", "token := x", ""), false, "git_diff_secret_preserved"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			res := compressor.CompressExit([]byte(tc.in), compressor.ModeSafe, 0)
			if tc.applied {
				if !res.Applied {
					t.Fatalf("want compacted, got reason=%q", res.Reason)
				}
				for _, l := range strings.Split(tc.in, "\n") {
					if (strings.HasPrefix(l, "+") || strings.HasPrefix(l, "-")) && !strings.Contains(string(res.Output), l+"\n") {
						t.Fatalf("change line %q lost in %q", l, res.Output)
					}
				}
				return
			}
			if res.Applied || string(res.Output) != tc.in {
				t.Fatalf("want raw, got applied=%v reason=%q", res.Applied, res.Reason)
			}
			if res.Reason != tc.reason {
				t.Fatalf("reason=%q, want %q", res.Reason, tc.reason)
			}
		})
	}
}
