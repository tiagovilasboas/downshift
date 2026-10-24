// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package telemetry

import (
	"strings"
	"testing"
	"time"
)

// renderFixture is one session with one row of each kind: a model move that
// finished, an effort-only change, a held spawn and an unchanged spawn.
func renderFixture() []Event {
	moved := td("d1", sessA, 0, "claude-haiku-5-5", ReasonClassifierConfident)
	effort := Event{
		CorrelationID: "d2", Timestamp: at(time.Minute), SessionID: sessA, Harness: "claude-code",
		Outcome: OutcomeRewriteEmitted, Tier: "mid", Verdict: "DOWNSHIFT",
		FromModel: "claude-haiku-5-5", ToModel: "claude-haiku-5-5",
		RequestedEffort: "high", FinalEffort: "low",
	}
	held := td("d3", sessA, 2*time.Minute, "claude-sonnet-5-5", ReasonClassifierUnconfident, "HELD_UNCONFIDENT_DOWNSHIFT")
	held.Outcome = OutcomeAllow
	held.FromModel = "claude-sonnet-5-5"
	fit := td("d4", sessA, 3*time.Minute, "claude-haiku-5-5")
	fit.Outcome = OutcomeAllow
	fit.FromModel = "claude-haiku-5-5"
	return []Event{
		moved, tuse("d1", sessA, 30*time.Second, "claude-haiku-5-5", 0.05, 0.30),
		effort, tend("d2", sessA, ChildCompleted, 2*time.Minute+30*time.Second),
		held, tend("d3", sessA, ChildCompleted, 3*time.Minute+30*time.Second),
		fit, tend("d4", sessA, ChildCompleted, 4*time.Minute),
	}
}

// TestSessionTree_GoldenPlainWords pins the text a first-time user reads. The
// ALL_CAPS codes and the complexity word are not in it.
func TestSessionTree_GoldenPlainWords(t *testing.T) {
	const want = "session · 4 subagents · 1 model changed · 1 effort changed · 1 kept as requested (why: --why) · saved 100% of work · 100% of cache\n" +
		"├─ subagent 1  haiku   ✓  10k tokens · saved from opus  moved to haiku\n" +
		"├─ subagent 2  haiku   ✓  -                             effort lowered to low\n" +
		"├─ subagent 3  sonnet  ✓  -                             kept: unsure of the task\n" +
		"└─ subagent 4  haiku   ✓  -                             left unchanged, already fit\n"
	got := SessionTree(renderFixture(), sessA, TreeOptions{})
	if got != want {
		t.Fatalf("golden mismatch\n--- got\n%s--- want\n%s", got, want)
	}
	for _, banned := range []string{"MEDIUM", "HELD_", "CLASSIFIER", "switched", "upshift"} {
		if strings.Contains(got, banned) {
			t.Errorf("default view must not carry %q:\n%s", banned, got)
		}
	}
}

func TestEffortWords(t *testing.T) {
	cases := []struct{ from, to, want string }{
		{"low", "high", "effort raised to high"},
		{"high", "low", "effort lowered to low"},
		{"high", "high", "effort changed to high"},
		{"unknown", "high", "effort changed to high"},
		{"low", "", "effort changed"},
	}
	for _, c := range cases {
		if got := effortWords(c.from, c.to); got != c.want {
			t.Errorf("effortWords(%q, %q) = %q, want %q", c.from, c.to, got, c.want)
		}
	}
}

// Every HELD_ code that why.go explains has a short clause for the tree row.
func TestHeldWords_CoversEveryHeldCode(t *testing.T) {
	for code := range reasonWords {
		if strings.HasPrefix(code, "HELD_") {
			if _, ok := heldWords[code]; !ok {
				t.Errorf("no tree clause for %s", code)
			}
		}
	}
}

// The JSON reason column is unchanged: it still carries the codes.
func TestSessionTree_JSONReasonKeepsCodes(t *testing.T) {
	view := BuildSessionTree(renderFixture(), sessA, TreeOptions{})
	if r := view.Rows[0].Reason; !strings.Contains(r, "MEDIUM") || !strings.Contains(r, "confident") {
		t.Fatalf("json reason changed: %q", r)
	}
}
