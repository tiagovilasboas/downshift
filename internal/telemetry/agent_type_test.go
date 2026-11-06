// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package telemetry_test

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

// The agent type is a label that ends up in `stats --export`, which people paste
// into public issues. Built-in names are kept; anything else is a name the user
// chose and could say something about their work, so it becomes "custom".
func TestAgentTypeLabel(t *testing.T) {
	for raw, want := range map[string]string{
		"":                       "unspecified",
		"general-purpose":        "general-purpose",
		"Explore":                "Explore",
		"Plan":                   "Plan",
		"claude":                 "claude",
		"statusline-setup":       "statusline-setup",
		"claude-code-guide":      "claude-code-guide",
		"payments-fraud-auditor": "custom",
		"explore":                "custom", // names are case-sensitive in Claude Code
		"Explore ":               "custom",
	} {
		if got := telemetry.AgentTypeLabel(raw); got != want {
			t.Errorf("AgentTypeLabel(%q) = %q, want %q", raw, got, want)
		}
	}
}

func decision(agentType, from string) telemetry.Event {
	return telemetry.Event{Harness: "claude-code", Agent: "subagent", Outcome: telemetry.OutcomeRewriteEmitted, AgentType: agentType, FromModel: from, ToModel: "claude-haiku-5-5", Verdict: "DOWNSHIFT"}
}

// The point of the field: say how many decisions of each type had no known
// baseline, so a gap like "Explore and Plan were never routed" is a number.
func TestAggregateCountsUnknownBaselineByAgentType(t *testing.T) {
	s := telemetry.Aggregate([]telemetry.Event{
		decision("general-purpose", "claude-sonnet-5-5"),
		decision("Explore", "unknown"),
		decision("Explore", "unknown"),
		decision("Explore", "claude-sonnet-5-5"),
		decision("custom", "unknown"),
		// An event from before the field existed: not counted per type.
		decision("", "unknown"),
		{Harness: "claude-code", Outcome: telemetry.OutcomeUsage, AgentType: "Explore"}, // usage records are not decisions
	})
	explore := s.ByAgentType["Explore"]
	if explore.Decisions != 3 || explore.UnknownBaseline != 2 {
		t.Errorf("Explore = %+v, want 3 decisions, 2 unknown", explore)
	}
	if gp := s.ByAgentType["general-purpose"]; gp.Decisions != 1 || gp.UnknownBaseline != 0 {
		t.Errorf("general-purpose = %+v", gp)
	}
	if _, ok := s.ByAgentType[""]; ok {
		t.Errorf("events without a type must not create an empty bucket: %v", s.ByAgentType)
	}
}

func TestExportCarriesAgentTypesWithoutRawNames(t *testing.T) {
	now := time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC)
	ev := decision("Explore", "unknown")
	ev.Timestamp = now.Add(-time.Hour).Format(time.RFC3339Nano)
	sum := telemetry.ExportSummary([]telemetry.Event{ev}, 7, now)
	raw, _ := json.Marshal(sum)
	if !strings.Contains(string(raw), `"agent_types":{"Explore":{"decisions":1,"unknown_baseline":1}}`) {
		t.Errorf("export lacks the per-type counts: %s", raw)
	}
	// A summary of events with no type keeps the older shape.
	plain := telemetry.Event{Harness: "claude-code", Agent: "subagent", Outcome: telemetry.OutcomeRewriteEmitted, FromModel: "unknown", Timestamp: ev.Timestamp}
	raw, _ = json.Marshal(telemetry.ExportSummary([]telemetry.Event{plain}, 7, now))
	if strings.Contains(string(raw), "agent_types") {
		t.Errorf("no types recorded, field must be omitted: %s", raw)
	}
}

func printDetail(evs []telemetry.Event) string {
	var b strings.Builder
	telemetry.PrintStats(evs, telemetry.StatsOptions{Detail: true}, &b)
	return b.String()
}

func TestPrintStatsDetail_ShowsUnknownBaselineByAgentType(t *testing.T) {
	out := printDetail([]telemetry.Event{
		decision("general-purpose", "claude-sonnet-5-5"),
		decision("Explore", "unknown"),
		decision("Explore", "unknown"),
		decision("Explore", "claude-sonnet-5-5"),
		decision("custom", "unknown"),
	})
	i := strings.Index(out, "Baseline by agent type")
	if i < 0 {
		t.Fatalf("no per-type section:\n%s", out)
	}
	section := out[i:]
	for _, want := range []string{"Explore", "3 decisions", "2 unknown", "general-purpose", "custom"} {
		if !strings.Contains(section, want) {
			t.Errorf("section is missing %q:\n%s", want, section)
		}
	}
	// Most unknown first, so the type that needs attention leads.
	if strings.Index(section, "Explore") > strings.Index(section, "general-purpose") {
		t.Errorf("types should be ordered by unknown baseline, highest first:\n%s", section)
	}
}

func TestPrintStatsDetail_OmitsTheSectionWithoutTypes(t *testing.T) {
	plain := telemetry.Event{Harness: "claude-code", Agent: "subagent", Outcome: telemetry.OutcomeRewriteEmitted, FromModel: "unknown", Verdict: "UNKNOWN"}
	if out := printDetail([]telemetry.Event{plain}); strings.Contains(out, "Baseline by agent type") {
		t.Errorf("events from before the field must not print an empty section:\n%s", out)
	}
}
