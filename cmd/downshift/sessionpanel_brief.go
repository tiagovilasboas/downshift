// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import "github.com/tiagovilasboas/downshift/internal/telemetry"

// panelBrief is `session --panel --brief`: exactly what the agents band draws.
// The session counts, the summary and the spawn rows are the PanelView's
// values (a test holds them equal); the token text, by_agent_type and the
// per-spawn tokens are left out. The report figure is there only when the
// report was built (tokens_saved_estimate, tokens_saved_short).
type panelBrief struct {
	Session           string       `json:"session"`
	Summary           string       `json:"summary"`
	Switched          int          `json:"switched"`
	ModelSwitched     int          `json:"model_switched"`
	EffortChanged     int          `json:"effort_changed"`
	Held              int          `json:"held"`
	HeldGuardrail     int          `json:"held_guardrail"`
	UpshiftNotApplied int          `json:"upshift_not_applied"`
	Observed          int          `json:"observed"`
	Spawns            []briefSpawn `json:"spawns"`
	// TokensSavedEstimate is a pointer so a report of 0 is written, and an
	// unbuilt report is left out.
	TokensSavedEstimate *int64 `json:"tokens_saved_estimate,omitempty"`
	TokensSavedShort    string `json:"tokens_saved_short,omitempty"`
}

// briefSpawn is one spawn row of the band.
type briefSpawn struct {
	ToolUse       string `json:"tool_use,omitempty"`
	State         string `json:"state"`
	FromModel     string `json:"from_model"`
	ToModel       string `json:"to_model"`
	ObservedModel string `json:"observed_model,omitempty"`
	Label         string `json:"label"`
	SavedTotal    int64  `json:"saved_total"`
	SavedShort    string `json:"saved_short"`
}

// briefOf is the band's view without a report.
func briefOf(v telemetry.PanelView) panelBrief {
	b := panelBrief{
		Session: v.Session, Summary: v.Summary,
		Switched: v.Switched, ModelSwitched: v.ModelSwitched, EffortChanged: v.EffortChanged,
		Held: v.Held, HeldGuardrail: v.HeldGuardrail, UpshiftNotApplied: v.UpshiftNotApplied,
		Observed: v.Observed, Spawns: make([]briefSpawn, 0, len(v.Spawns)),
	}
	for _, sp := range v.Spawns {
		b.Spawns = append(b.Spawns, briefSpawn{
			ToolUse: sp.ToolUse, State: sp.State, FromModel: sp.FromModel, ToModel: sp.ToModel,
			ObservedModel: sp.ObservedModel, Label: sp.Label,
			SavedTotal: sp.SavedTotal, SavedShort: sp.SavedShort,
		})
	}
	return b
}

// briefWithTokens is briefOf plus the report's final figure and its short text.
func briefWithTokens(v telemetry.PanelView, tokensSaved int64) panelBrief {
	b := briefOf(v)
	b.TokensSavedEstimate = &tokensSaved
	b.TokensSavedShort = telemetry.TokensShort(tokensSaved)
	return b
}
