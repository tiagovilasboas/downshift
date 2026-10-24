// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package api

import (
	"net/http"

	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

// e2eStubEnabled reports whether the request asked for deterministic stub data
// with the ?e2e=1 query flag. It exists so the dashboard can be driven by a
// Playwright harness (B-W2 / DASH-2) against a fixed render, independent of the
// user's real events.jsonl. This is a test affordance only: it never changes
// the data a normal dashboard request returns.
func e2eStubEnabled(r *http.Request) bool {
	return r.URL.Query().Get("e2e") == "1"
}

// e2eSessionsStub returns a fixed sessions listing for the E2E harness. Values
// are hand-picked so the rendered dashboard is deterministic: two sessions,
// two harnesses, known spawn/switch/token counts.
func e2eSessionsStub() sessionsResponse {
	return sessionsResponse{
		Sessions: []sessionSummary{
			{
				SessionID:   "e2e0000000000000000000000000000000000000000000000000000000000001",
				Harness:     "claude-code",
				FirstSeen:   "2026-10-22T18:00:00Z",
				LastSeen:    "2026-10-22T18:30:00Z",
				SpawnCount:  4,
				Switched:    3,
				Held:        1,
				TokensSaved: 48000,
			},
			{
				SessionID:   "e2e0000000000000000000000000000000000000000000000000000000000002",
				Harness:     "codex",
				FirstSeen:   "2026-10-22T17:00:00Z",
				LastSeen:    "2026-10-22T17:20:00Z",
				SpawnCount:  2,
				Switched:    1,
				Held:        1,
				TokensSaved: 12000,
			},
		},
		Harnesses: []string{"claude-code", "codex"},
	}
}

// e2ePanelStub returns a fixed current-session panel for the E2E harness. It
// mirrors the shape writePanel produces, including a compactor block, so every
// dashboard tab has content to render deterministically.
func e2ePanelStub() sessionPanelResponse {
	tokens := telemetry.TokenSaving{
		Children:      4,
		SavedChildren: 3,
		WorkTokens:    60000,
		WorkSaved:     42000,
		CacheTokens:   20000,
		CacheSaved:    6000,
	}
	return sessionPanelResponse{
		PanelView: telemetry.PanelView{
			Session:       "e2e0000000000000000000000000000000000000000000000000000000000001",
			Switched:      3,
			ModelSwitched: 2,
			EffortChanged: 1,
			Held:          1,
			HeldGuardrail: 1,
			Observed:      4,
			Tokens:        tokens,
			TokensLine:    "work 70% · cache 30% (3 of 4 agents)",
			ByAgentType: []telemetry.PanelAgentType{
				{
					AgentType: "general-purpose",
					Tokens:    tokens,
					Line:      "general-purpose: work 70% · cache 30% (3 of 4 agents)",
				},
			},
			Spawns: []telemetry.PanelSpawn{
				{
					State:       "switched",
					FromModel:   "opus",
					ToModel:     "sonnet",
					Label:       "opus → sonnet (mid)",
					Tokens:      tokens,
					TokensLabel: "48k tokens · saved from opus",
				},
			},
		},
		Compactor: &compactorWire{
			Scope:                "all",
			Commands:             12,
			Compacted:            9,
			RawBytes:             480000,
			SavedBytes:           360000,
			EstimatedTokensSaved: 90000,
			TokenEstimate:        "~90k tokens saved",
		},
		TokensSavedEstimate: 138000,
	}
}
