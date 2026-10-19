// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package telemetry

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

// PanelView is the prompt-free record a panel mod reads (`downshift session
// --panel`). It is built here so the mod only draws: the labels are final text.
// No task text, path or raw id is in it; spawns are keyed by the hashed tool
// call id (see ToolUseKey).
type PanelView struct {
	Session  string `json:"session"`
	Switched int    `json:"switched"`
	Held     int    `json:"held"`
	Observed int    `json:"observed"`
	// SavedUSD is the signed real delta of priced children (baseline minus
	// actual, list prices): an estimate. HasSaved is false until a child is
	// priced; a reader then draws nothing about money.
	SavedUSD  float64      `json:"saved_usd"`
	HasSaved  bool         `json:"has_saved"`
	Estimated bool         `json:"estimated"` // always true: list-price estimate
	Spawns    []PanelSpawn `json:"spawns"`
}

// PanelSpawn is one subagent decision of the session.
type PanelSpawn struct {
	ToolUse   string `json:"tool_use,omitempty"` // ToolUseKey of the harness tool call id
	State     string `json:"state"`
	FromModel string `json:"from_model"`
	ToModel   string `json:"to_model"`
	Label     string `json:"label"` // e.g. "opus → sonnet (mid)", "held: original model kept"
}

// ToolUseKey is the hash a decision event stores for a harness tool call id
// (the same as ToolUseHash on the event): lowercase hex sha256 of the fixed
// prefix and the id. A reader that knows the raw id computes the same key.
func ToolUseKey(id string) string {
	sum := sha256.Sum256([]byte("harness-downshift-session-v1:" + id))
	return hex.EncodeToString(sum[:])
}

// PanelLabel is the short phrase for one spawn.
func PanelLabel(r SpawnRecord, ev Event) string {
	switch r.State {
	case StateSwitched:
		if r.FromModel == r.ToModel {
			return "effort changed (" + strings.ToLower(ev.Tier) + ")"
		}
		return shortOrQ(r.FromModel) + " → " + shortOrQ(r.ToModel) + " (" + strings.ToLower(ev.Tier) + ")"
	case StateHeld:
		return "held: original model kept"
	case StatePaused:
		return "paused (unrouted)"
	case StateControl:
		return "control (unrouted)"
	case StateBlocked:
		return "blocked: respawn on another model"
	}
	return "kept"
}

// BuildPanel folds one session's events into the panel record.
func BuildPanel(events []Event, sessionHash string) PanelView {
	v := PanelView{Estimated: true, Spawns: []PanelSpawn{}}
	if len(sessionHash) >= 8 {
		v.Session = sessionHash[:8]
	}
	for _, ev := range sessionEvents(events, sessionHash) {
		if !IsDecisionRow(ev) {
			continue
		}
		r := NewSpawnRecord(ev)
		switch r.State {
		case StateSwitched:
			v.Switched++
		case StateHeld:
			v.Held++
		case StateObserved:
			v.Observed++
		}
		v.Spawns = append(v.Spawns, PanelSpawn{
			ToolUse: ev.ToolUseHash, State: r.State, FromModel: r.FromModel, ToModel: r.ToModel, Label: PanelLabel(r, ev),
		})
	}
	s := SessionSummary(events, sessionHash)
	v.HasSaved, v.SavedUSD = s.HasCost, s.DeltaUSD
	return v
}
