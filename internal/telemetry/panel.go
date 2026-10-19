// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package telemetry

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
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
	// Tokens is the session's tokens saved from the requested model, work
	// (input+output) and cache counted apart; TokensLine is its final text, ""
	// until a child's tokens are known. No dollars: those are `downshift
	// report`'s, priced at report time.
	Tokens     TokenSaving `json:"tokens"`
	TokensLine string      `json:"tokens_line"`
	// ByAgentType is the same per agent type, with its final text.
	ByAgentType []PanelAgentType `json:"by_agent_type"`
	Spawns      []PanelSpawn     `json:"spawns"`
}

// PanelAgentType is one agent type's tokens saved in the session.
type PanelAgentType struct {
	AgentType string      `json:"agent_type"`
	Tokens    TokenSaving `json:"tokens"`
	Line      string      `json:"line"` // e.g. "general-purpose: work 70% · cache 64% (3 of 4 agents)"
}

// PanelSpawn is one subagent decision of the session.
type PanelSpawn struct {
	ToolUse   string `json:"tool_use,omitempty"` // ToolUseKey of the harness tool call id
	State     string `json:"state"`
	FromModel string `json:"from_model"`
	ToModel   string `json:"to_model"`
	Label     string `json:"label"` // e.g. "opus → sonnet (mid)", "held: original model kept"
	// Tokens is this agent's tokens once it ended; TokensLabel its text, e.g.
	// "48k tokens · saved from opus" ("" while it runs).
	Tokens      TokenSaving `json:"tokens"`
	TokensLabel string      `json:"tokens_label"`
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
	v := PanelView{Spawns: []PanelSpawn{}, ByAgentType: []PanelAgentType{}}
	if len(sessionHash) >= 8 {
		v.Session = sessionHash[:8]
	}
	mine := sessionEvents(events, sessionHash)
	saved := TokenSavings(mine)
	v.Tokens, v.TokensLine = saved.Total, saved.Total.Line()
	v.ByAgentType = AgentTypeLines(saved)
	for _, ev := range mine {
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
		child := saved.Child(ev)
		v.Spawns = append(v.Spawns, PanelSpawn{
			ToolUse: ev.ToolUseHash, State: r.State, FromModel: r.FromModel, ToModel: r.ToModel, Label: PanelLabel(r, ev),
			Tokens: child, TokensLabel: ChildTokensLabel(child, ev),
		})
	}
	return v
}

// ChildTokensLabel is one agent's token text: "48k tokens · saved from opus",
// "48k tokens · pricier than opus", "48k tokens" when it ran as requested, or
// "" before its tokens are known.
func ChildTokensLabel(c TokenSaving, d Event) string {
	if !c.Observed() {
		return ""
	}
	out := formatTokens(c.WorkTokens+c.CacheTokens) + " tokens"
	switch c.Direction() {
	case TokenCheaper:
		out += " · saved from " + shortOrQ(d.FromModel)
	case TokenPricier:
		out += " · pricier than " + shortOrQ(d.FromModel)
	}
	return out
}

// AgentTypeLines is one row per agent type with its AgentTypeLine, ordered by
// tokens, most first.
func AgentTypeLines(v TokenSavingView) []PanelAgentType {
	out := make([]PanelAgentType, 0, len(v.ByAgentType))
	for typ, s := range v.ByAgentType {
		out = append(out, PanelAgentType{AgentType: typ, Tokens: s, Line: AgentTypeLine(typ, s)})
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i].Tokens, out[j].Tokens
		if ta, tb := a.WorkTokens+a.CacheTokens, b.WorkTokens+b.CacheTokens; ta != tb {
			return ta > tb
		}
		return out[i].AgentType < out[j].AgentType
	})
	return out
}

// AgentTypeLine is "general-purpose: work 70% · cache 64% (3 of 4 agents)".
func AgentTypeLine(typ string, s TokenSaving) string {
	noun := "agents"
	if s.Children == 1 {
		noun = "agent"
	}
	return fmt.Sprintf("%s: work %s · cache %s (%d of %d %s)", typ, percentText(s.WorkPercent()), percentText(s.CachePercent()), s.SavedChildren, s.Children, noun)
}
