// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package accounting

import (
	"testing"

	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

func TestBuildSessionSummary_Empty(t *testing.T) {
	summary := BuildSessionSummary(nil, "")
	if summary.SessionID != "" {
		t.Errorf("expected empty session id, got %q", summary.SessionID)
	}
	if len(summary.Spawns) != 0 {
		t.Errorf("expected empty spawns, got %d", len(summary.Spawns))
	}
}

func TestBuildSessionSummary_WithEvents(t *testing.T) {
	events := []telemetry.Event{
		{
			Harness:     "claude-code",
			Outcome:     telemetry.OutcomeRewriteEmitted,
			FromModel:   "claude-opus-4",
			ToModel:     "claude-sonnet-4",
			Tier:        "MID",
			SessionID:   telemetry.HashSessionID("test-session"),
			ToolUseHash: "abc123",
		},
	}
	hash := telemetry.HashSessionID("test-session")

	summary := BuildSessionSummary(events, hash)

	if summary.Routing.Switched != 1 {
		t.Errorf("expected 1 switched, got %d", summary.Routing.Switched)
	}
	if len(summary.Spawns) != 1 {
		t.Errorf("expected 1 spawn, got %d", len(summary.Spawns))
	}
}

func TestBuildGlobalSummary_Empty(t *testing.T) {
	summary := BuildGlobalSummary(nil, SummaryOptions{Days: 7})
	if summary.Days != 7 {
		t.Errorf("expected 7 days, got %d", summary.Days)
	}
	if summary.Routing.Switched != 0 {
		t.Errorf("expected 0 switched, got %d", summary.Routing.Switched)
	}
}

func TestBuildComparisonReport_Empty(t *testing.T) {
	report := BuildComparisonReport(nil, 7)
	if report.Days != 7 {
		t.Errorf("expected 7 days, got %d", report.Days)
	}
	if report.Enough {
		t.Error("expected not enough data")
	}
}

func TestRoutingMetrics_ZeroValues(t *testing.T) {
	r := RoutingMetrics{}
	if r.Switched != 0 || r.Held != 0 || r.WorkTokens != 0 {
		t.Error("expected all zeros")
	}
}

func TestAgentTypesFromPanel_SingleUntyped(t *testing.T) {
	panel := telemetry.PanelView{
		ByAgentType: []telemetry.PanelAgentType{
			{AgentType: telemetry.UntypedAgent},
		},
	}
	types := agentTypesFromPanel(panel)
	if types != nil {
		t.Error("expected nil for single untyped")
	}
}

func TestAgentTypesFromPanel_Multiple(t *testing.T) {
	panel := telemetry.PanelView{
		ByAgentType: []telemetry.PanelAgentType{
			{AgentType: "general-purpose"},
			{AgentType: "code-review"},
		},
	}
	types := agentTypesFromPanel(panel)
	if len(types) != 2 {
		t.Errorf("expected 2 types, got %d", len(types))
	}
}
