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
	Session string `json:"session"`
	// Switched counts the spawns Downshift changed: a model or an effort.
	// ModelSwitched and EffortChanged split it: the model differs from the
	// requested one, or the model is the same and only the effort changed.
	Switched      int `json:"switched"`
	ModelSwitched int `json:"model_switched"`
	EffortChanged int `json:"effort_changed"`
	Held          int `json:"held"`
	// HeldGuardrail is the held spawns kept by a guardrail (R1, R5, R7, R8,
	// HELD_*) or the credit gate; UpshiftNotApplied counts the UPSHIFT verdicts
	// that did not change the spawn. Held stays the sum of the held spawns.
	HeldGuardrail     int `json:"held_guardrail"`
	UpshiftNotApplied int `json:"upshift_not_applied"`
	Observed          int `json:"observed"`
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
	// ObservedModel is the model the child was observed to run on (its
	// resolved or usage record), set only when it is known and differs from
	// ToModel. Empty means "as decided" or "not observed".
	ObservedModel string `json:"observed_model,omitempty"`
	Label         string `json:"label"` // e.g. "opus → sonnet (mid)", "held: original model kept"
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
	observed := observedModels(mine)
	for _, ev := range mine {
		if !IsDecisionRow(ev) {
			continue
		}
		r := NewSpawnRecord(ev)
		switch r.State {
		case StateSwitched:
			v.Switched++
			if switchesModel(r.FromModel, r.ToModel) {
				v.ModelSwitched++
			} else {
				v.EffortChanged++
			}
		case StateHeld:
			v.Held++
		}
		if UpshiftNotApplied(ev) {
			v.UpshiftNotApplied++
		}
		if HeldByGuardrail(ev) {
			v.HeldGuardrail++
		}
		switch r.State {
		case StateObserved:
			v.Observed++
		}
		child := saved.Child(ev)
		v.Spawns = append(v.Spawns, PanelSpawn{
			ToolUse: ev.ToolUseHash, State: r.State, FromModel: r.FromModel, ToModel: r.ToModel,
			ObservedModel: differentModel(observed[ev.Harness+"\n"+DecisionKey(ev)], r.ToModel),
			Label:         PanelLabel(r, ev),
			Tokens:        child, TokensLabel: ChildTokensLabel(child, ev),
		})
	}
	return v
}

// observedModels maps each decision (harness and DecisionKey) to the model its
// child was observed to run on. The resolved record (what the harness chose at
// launch) wins over a usage record (what the transcript reported). Only known
// ids are kept.
func observedModels(events []Event) map[string]string {
	out := map[string]string{}
	for _, ev := range events {
		if ev.LinkedDecision == "" || !knownModel(ev.ToModel) {
			continue
		}
		key := ev.Harness + "\n" + ev.LinkedDecision
		switch ev.Outcome {
		case OutcomeResolved:
			out[key] = ev.ToModel
		case OutcomeUsage:
			if _, seen := out[key]; !seen {
				out[key] = ev.ToModel
			}
		}
	}
	return out
}

// differentModel returns observed when it is known and is not the decided
// model, "" otherwise. A dated id (claude-haiku-4-5-20251001) is the same
// model as its undated form (claude-haiku-4-5).
func differentModel(observed, decided string) string {
	if !knownModel(observed) || sameModel(observed, decided) {
		return ""
	}
	return observed
}

// sameModel compares two model ids, ignoring a trailing 8-digit date stamp.
func sameModel(a, b string) bool {
	return undatedModel(a) == undatedModel(b)
}

func undatedModel(id string) string {
	if i := strings.LastIndexByte(id, '-'); i >= 0 && len(id)-i-1 == 8 && isDigits(id[i+1:]) {
		return id[:i]
	}
	return id
}

func isDigits(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return s != ""
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

// SavedTypeLines is the AgentTypeLine of every type that saved something, or
// nil when there is nothing worth a line: one untyped group repeats the total,
// and a row of zeros is noise.
func SavedTypeLines(types []PanelAgentType) []string {
	var out []string
	for _, t := range types {
		if t.Tokens.WorkSaved+t.Tokens.CacheSaved > 0 {
			out = append(out, t.Line)
		}
	}
	if len(out) == 1 && len(types) == 1 && types[0].AgentType == UntypedAgent {
		return nil
	}
	return out
}

// AgentTypeLine is "general-purpose: 70% work · 64% cache".
func AgentTypeLine(typ string, s TokenSaving) string {
	return fmt.Sprintf("%s: %s work · %s cache", typ, percentText(s.WorkPercent()), percentText(s.CachePercent()))
}
