// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package telemetry

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// heldWords is the short plain-words clause for a held row: why the spawn was
// kept as requested. Every HELD_* code in reason_codes.go has one (a test
// enforces it). The long sentences in why.go are too wide for a tree row.
var heldWords = map[string]string{
	"HELD_UNCONFIDENT_DOWNSHIFT":        "unsure of the task",
	"HELD_COMPLEX_DOWNSHIFT":            "complex task stays on its model",
	"HELD_FOREIGN_MODEL":                "target model belongs to another harness",
	"HELD_RISK_DOWNSHIFT":               "risk area, cheaper model not used",
	"HELD_UNCONFIDENT_UNKNOWN_SMALL":    "unsure, requested model unknown",
	"HELD_UNCONFIDENT_UNKNOWN_FRONTIER": "unsure, requested model unknown",
	"HELD_UNCONFIDENT_UNKNOWN_MID":      "unsure, requested model unknown",
	"HELD_UNCONFIDENT_UPSHIFT":          "unsure, bigger model not paid for",
	ReasonGuessHeld:                     "baseline was guessed, only a small downshift",
	ReasonHeldUnmapped:                  "a guardrail this version cannot name",
}

// rowWords is the plain-words reason column of one tree row: what Downshift
// did to this subagent. ALL_CAPS reason codes stay in the JSON and in --why.
func rowWords(r TreeRow) string {
	switch r.State {
	case StateBlocked:
		return "blocked, asked to respawn on another model"
	case StatePaused:
		return "paused, left unchanged"
	case StateControl:
		return "control group, left unchanged"
	case StateObserved:
		return "left unchanged, already fit"
	case StateHeld:
		return "kept: " + heldClause(r.ReasonCodes)
	}
	if r.switchesModel {
		return "moved to " + r.Model
	}
	return effortWords(r.effortFrom, r.effortTo)
}

// heldClause is the first held code's clause, else a generic one.
func heldClause(codes []string) string {
	for _, c := range codes {
		if w, ok := heldWords[c]; ok {
			return w
		}
	}
	if hasCode(codes, ReasonCreditHeld) {
		return "quota gate held the change"
	}
	return "kept as requested"
}

// effortLevels orders the effort vocabulary so a change can read as raised or
// lowered. An unknown level is not ranked.
var effortLevels = map[string]int{"low": 1, "medium": 2, "high": 3, "xhigh": 4, "max": 5}

// effortWords reads an effort-only change: "effort raised to high".
func effortWords(from, to string) string {
	if to == "" || to == "unknown" {
		return "effort changed"
	}
	rf, rt := effortLevels[from], effortLevels[to]
	switch {
	case rf == 0 || rt == 0 || rf == rt:
		return "effort changed to " + to
	case rt > rf:
		return "effort raised to " + to
	default:
		return "effort lowered to " + to
	}
}

// headerWords is the header's middle part for a session with subagents: what
// changed and what was kept, in words. The codes behind it are in --why.
func headerWords(v SessionTreeView) []string {
	var out []string
	switch v.ModelSwitched {
	case 0:
		out = append(out, "no model changed")
	case 1:
		out = append(out, "1 model changed")
	default:
		out = append(out, fmt.Sprintf("%d models changed", v.ModelSwitched))
	}
	if v.EffortChanged > 0 {
		out = append(out, fmt.Sprintf("%d effort changed", v.EffortChanged))
	}
	if v.HeldGuardrail > 0 {
		out = append(out, fmt.Sprintf("%d kept as requested (why: --why)", v.HeldGuardrail))
	}
	if v.UpshiftNotApplied > 0 {
		out = append(out, fmt.Sprintf("%d bigger model not applied", v.UpshiftNotApplied))
	}
	return out
}

// Render lays the view out as text. Columns are aligned: tree and label, model,
// status, tokens, reason; a last line gives tokens saved per agent type.
func (v SessionTreeView) Render() string {
	noun := "subagents"
	if v.Subagents == 1 {
		noun = "subagent"
	}
	head := []string{"session"}
	if v.Subagents == 0 {
		head = append(head, "no subagents yet")
	} else {
		head = append(head, fmt.Sprintf("%d %s", v.Subagents, noun))
		head = append(head, headerWords(v)...)
		if line := v.Tokens.Line(); line != "" {
			head = append(head, line)
		} else {
			head = append(head, "tokens not observed")
		}
	}
	if v.Paused {
		head = append(head, "routing paused for this session")
	}
	var b strings.Builder
	b.WriteString(strings.Join(head, " · "))
	b.WriteByte('\n')

	names := make([]string, len(v.Rows))
	costs := make([]string, len(v.Rows))
	nameW, modelW, costW := 0, 0, 0
	var lastAt []bool // per ancestor depth: that ancestor was the last of its siblings
	for i, r := range v.Rows {
		last := true
		for _, n := range v.Rows[i+1:] {
			if n.Depth < r.Depth {
				break
			}
			if n.Depth == r.Depth {
				last = false
				break
			}
		}
		lastAt = append(lastAt[:r.Depth], last)
		var p strings.Builder
		for d := 0; d < r.Depth; d++ {
			if lastAt[d] {
				p.WriteString("   ")
			} else {
				p.WriteString("│  ")
			}
		}
		if last {
			p.WriteString("└─ ")
		} else {
			p.WriteString("├─ ")
		}
		names[i] = p.String() + r.Label
		nameW = max(nameW, utf8.RuneCountInString(names[i]))
		modelW = max(modelW, utf8.RuneCountInString(r.Model))
		costs[i] = "-"
		if r.TokensLabel != "" {
			costs[i] = r.TokensLabel
		}
		costW = max(costW, utf8.RuneCountInString(costs[i]))
	}
	for i, r := range v.Rows {
		line := fmt.Sprintf("%s  %s  %s  %s  %s",
			padRight(names[i], nameW), padRight(r.Model, modelW), r.mark,
			padRight(costs[i], costW), rowWords(r))
		b.WriteString(strings.TrimRight(line, " "))
		b.WriteByte('\n')
	}
	if lines := SavedTypeLines(v.ByAgentType); len(lines) > 0 {
		b.WriteString("by type: " + strings.Join(lines, " · ") + "\n")
	}
	return b.String()
}

func padRight(s string, w int) string {
	if n := utf8.RuneCountInString(s); n < w {
		return s + strings.Repeat(" ", w-n)
	}
	return s
}
