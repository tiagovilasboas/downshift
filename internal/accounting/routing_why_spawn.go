// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package accounting

import (
	"strings"

	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

// noteMissing is the cell text for a spawn whose decision holds no note.
const noteMissing = "note not recorded"

// SpawnWhy is one spawn of a session in `session --panel`: what was requested,
// what it became, and one short cell that says why. Ids and enums only.
type SpawnWhy struct {
	ToolUse   string `json:"tool_use,omitempty"`
	Requested string `json:"requested_model"`
	Final     string `json:"final_model"`
	Verdict   string `json:"verdict,omitempty"`
	Reason    string `json:"reason,omitempty"`
	Cell      string `json:"cell"`
}

// BuildSpawnWhy lists the decisions of one session, in log order. The caller
// passes the session's events; sessionHash selects them as BuildPanel does.
func BuildSpawnWhy(events []telemetry.Event, sessionHash string) []SpawnWhy {
	out := []SpawnWhy{}
	for _, ev := range scopeTelemetry(events, Scope{Kind: ScopeSession, SessionHash: sessionHash}) {
		if !telemetry.IsDecisionRow(ev) {
			continue
		}
		sw := SpawnWhy{ToolUse: ev.ToolUseHash, Requested: orUnknown(safeID(ev.FromModel)), Final: orUnknown(safeID(ev.ToModel))}
		cell := noteMissing
		if n, ok := noteOf(ev); ok {
			sw.Verdict, sw.Reason = n.Verdict, reasonKey(n)
			cell = sw.Reason
		}
		sw.Cell = sw.Requested + " → " + sw.Final + " · " + cell
		out = append(out, sw)
	}
	return out
}

func orUnknown(s string) string {
	if s == "" {
		return "?"
	}
	return s
}

// safeID keeps a model id printable: ids come from a harness payload, so
// anything outside the id alphabet or over 128 bytes is shown as unknown.
func safeID(id string) string {
	if len(id) > 128 {
		return ""
	}
	for _, r := range id {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || strings.ContainsRune("._:/+[],=-", r)) {
			return ""
		}
	}
	return id
}
