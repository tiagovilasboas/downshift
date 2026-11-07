// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import (
	"fmt"
	"strings"
	"testing"
)

func seqLines(format string, from, to int) []string {
	var l []string
	for i := from; i <= to; i++ {
		l = append(l, fmt.Sprintf(format, i))
	}
	return l
}

func TestFoldRepeats(t *testing.T) {
	ok := seqLines("test_case_%03d ... ok", 1, 20)
	tests := []struct {
		name    string
		in      string
		applied bool
		reason  string
		want    string
	}{
		{"run folded keeps 3 head and 2 tail", "head\n" + strings.Join(ok, "\n") + "\ntail\n", true, "similar lines folded",
			"head\n" + strings.Join(ok[:3], "\n") + "\n[... 15 similar lines folded]\n" + strings.Join(ok[18:], "\n") + "\ntail\n"},
		{"seven lines stay", strings.Join(ok[:7], "\n") + "\n", false, foldNoRun, ""},
		{"eight lines fold", strings.Join(seqLines("mod%d.ts is long enough line", 1, 8), "\n"), true, "similar lines folded", ""},
		{"danger word in every run line keeps it", strings.Join(seqLines("test_case_%03d ... FAILED", 1, 10), "\n"), false, foldNoRun, ""},
		{"warning inside run keeps it", strings.Join(seqLines("step %d warning ok", 1, 10), "\n") + "\n", false, foldNoRun, ""},
		{"secret in run keeps it", strings.Join(seqLines("load token_%d done", 1, 10), "\n") + "\n", false, foldNoRun, ""},
		{"different words break run", strings.Join(append(seqLines("alpha %d", 1, 4), seqLines("beta %d", 1, 4)...), "\n"), false, foldNoRun, ""},
		{"40 distinct file paths stay unfolded", strings.Join(seqLines("docs/fixtures/case%02d.md", 1, 40), "\n") + "\n", false, foldNoRun, ""},
		{"80 distinct file paths fold keeping 20 head and 10 tail", strings.Join(seqLines("docs/fixtures/case%03d.md", 1, 80), "\n") + "\n", true, "similar lines folded",
			strings.Join(seqLines("docs/fixtures/case%03d.md", 1, 20), "\n") + "\n[... 50 similar lines folded]\n" + strings.Join(seqLines("docs/fixtures/case%03d.md", 71, 80), "\n") + "\n"},
		{"empty input", "", false, foldNoRun, ""},
		{"short run is not shorter", strings.Join(seqLines("a%d", 1, 8), "\n"), false, genericNoGain, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := FoldRepeats([]byte(tc.in))
			if got.Applied != tc.applied || got.Reason != tc.reason || got.Format != FormatGeneric {
				t.Fatalf("applied=%v reason=%q format=%q, want %v %q generic", got.Applied, got.Reason, got.Format, tc.applied, tc.reason)
			}
			if !tc.applied && string(got.Output) != tc.in {
				t.Fatalf("output changed when not applied")
			}
			if tc.want != "" && string(got.Output) != tc.want {
				t.Fatalf("output:\n%s\nwant:\n%s", got.Output, tc.want)
			}
			if got.Applied && got.ReducedBytes != len(got.Output) {
				t.Fatalf("ReducedBytes %d != %d", got.ReducedBytes, len(got.Output))
			}
		})
	}
}
