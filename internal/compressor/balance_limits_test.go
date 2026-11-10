// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import (
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"
)

// A failure line over the line cap keeps its first 300 and last 200 bytes:
// the assertion that often ends it survives the cut. Any other long line
// keeps only its head.
func TestBalanceCutKeepsFailureLineEnd(t *testing.T) {
	cases := []struct {
		name, line string
		wantEnd    bool
	}{
		{"900-byte failure line keeps its end", "FAIL TestX " + strings.Repeat("a", 877) + "want=1 got=2", true},
		{"error word mid-line keeps its end", strings.Repeat("b", 400) + " error: " + strings.Repeat("c", 480) + "TAIL", true},
		{"plain line keeps only its head", strings.Repeat("d", 896) + "TAIL", false},
		{"multi-byte end stays valid", "panic: " + strings.Repeat("é", 450), true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out := balanceCut(tc.line)
			if !utf8.ValidString(out) {
				t.Fatalf("invalid UTF-8: %q", out)
			}
			head := tc.line[:300]
			for !utf8.ValidString(head) {
				head = head[:len(head)-1]
			}
			if !strings.HasPrefix(out, head) {
				t.Errorf("head lost: %q", out[:40])
			}
			tail := tc.line[len(tc.line)-198:]
			if got := strings.HasSuffix(out, tail); got != tc.wantEnd {
				t.Errorf("kept end = %v, want %v: %q", got, tc.wantEnd, out[len(out)-60:])
			}
			if len(out) >= len(tc.line) {
				t.Errorf("line not shortened: %d -> %d", len(tc.line), len(out))
			}
		})
	}
	if l := "FAIL " + strings.Repeat("x", 895); len(l) != 900 || !strings.HasSuffix(balanceCut(l), l[700:]) || !strings.HasPrefix(balanceCut(l), l[:300]) {
		t.Errorf("a 900-byte failure line must keep bytes 0-300 and 700-900")
	}
}

// Each limit of BalanceLimits changes the cut; a zero field keeps the default.
func TestBalanceWithLimits(t *testing.T) {
	var diff, matches, log strings.Builder
	diff.WriteString("diff --git a/f.go b/f.go\n--- a/f.go\n+++ b/f.go\n")
	for h := 0; h < 4; h++ {
		fmt.Fprintf(&diff, "@@ -%d,100 +%d,100 @@\n", h*100, h*100)
		for i := 0; i < 100; i++ {
			fmt.Fprintf(&diff, "+line %d\n", i)
		}
	}
	for f := 0; f < 4; f++ {
		for i := 0; i < 100; i++ {
			fmt.Fprintf(&matches, "f%d.go:%d:match\n", f, i)
		}
	}
	for i := 0; i < 1000; i++ {
		fmt.Fprintf(&log, "step %d\n", i)
	}
	count := strings.Count
	cases := []struct {
		name string
		kind BalanceKind
		text string
		lim  BalanceLimits
		want func(string) bool
	}{
		{"hunk lines", BalanceDiff, diff.String(), BalanceLimits{HunkLines: 5}, func(s string) bool { return count(s, "+line ") == 20 }},
		{"diff lines", BalanceDiff, diff.String(), BalanceLimits{DiffLines: 40}, func(s string) bool { return count(s, "+line ") == 40 }},
		{"matches per file", BalanceMatches, matches.String(), BalanceLimits{MatchesPerFile: 3}, func(s string) bool { return count(s, ":match") == 12 }},
		{"match lines", BalanceMatches, matches.String(), BalanceLimits{MatchLines: 30}, func(s string) bool { return count(s, ":match") == 30 }},
		{"log head", BalanceLog, log.String(), BalanceLimits{LogHead: 10}, func(s string) bool {
			return strings.HasPrefix(s, "step 0\n") && !strings.Contains(s, "step 10\n") && strings.Contains(s, "step 999")
		}},
		{"log tail", BalanceLog, log.String(), BalanceLimits{LogTail: 10}, func(s string) bool {
			return strings.Contains(s, "step 990\n") && !strings.Contains(s, "step 989\n")
		}},
		{"zero keeps default", BalanceLog, log.String(), BalanceLimits{}, func(s string) bool {
			return strings.Contains(s, "step 59\n") && !strings.Contains(s, "step 60\n")
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out, ok := BalanceWith(tc.kind, []byte(tc.text), tc.lim)
			if !ok || !tc.want(string(out)) {
				t.Errorf("ok=%v, cut does not follow the limit:\n%.600s", ok, out)
			}
		})
	}
	if got, want := (BalanceLimits{}).WithDefaults(), DefaultBalanceLimits(); got != want {
		t.Errorf("WithDefaults() = %+v, want %+v", got, want)
	}
}
