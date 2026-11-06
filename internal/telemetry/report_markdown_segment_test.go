// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package telemetry

import (
	"bytes"
	"strings"
	"testing"
)

// The markdown routed row is per harness; a cross-harness total never shows.
func TestPrintSegmentedReportMarkdown_NeverSummedAcrossHarnesses(t *testing.T) {
	seg := func(name string, spawns int) HarnessReport {
		return HarnessReport{Harness: name, Report: Report{Routed: GroupSummary{Spawns: spawns}}}
	}
	cases := []struct {
		name     string
		segments []HarnessReport
		want     []string
		notWant  []string
	}{
		{"two harnesses", []HarnessReport{seg("claude-code", 125), seg("codex", 1)},
			[]string{"| claude-code | 125 |", "| codex | 1 |", "| harness | spawns"}, []string{"| 126 |", "| routed |"}},
		{"one harness", []HarnessReport{seg("claude-code", 125)},
			[]string{"| routed | 125 |", "| group | spawns"}, []string{"| claude-code | 125 |"}},
		{"second empty", []HarnessReport{seg("claude-code", 7), seg("codex", 0)},
			[]string{"| claude-code | 7 |"}, []string{"| codex |"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var b bytes.Buffer
			PrintSegmentedReportMarkdown(&b, Report{}, c.segments)
			out := b.String()
			for _, w := range c.want {
				if !strings.Contains(out, w) {
					t.Errorf("missing %q in:\n%s", w, out)
				}
			}
			for _, n := range c.notWant {
				if strings.Contains(out, n) {
					t.Errorf("unexpected %q in:\n%s", n, out)
				}
			}
		})
	}
}
