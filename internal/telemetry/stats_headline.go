// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package telemetry

import (
	"fmt"
	"io"
	"sort"
	"strings"
)

// maxMixModels bounds the model-mix line; the rest folds into "other".
const maxMixModels = 5

// printHeadline writes the lines every `downshift stats` starts with: the
// tokens saved from the requested model (work and cache apart), the
// guardrails, the model mix and, when spawns name one, tokens saved per agent
// type. No dollars: `downshift report` prices tokens at report time.
func printHeadline(w io.Writer, s Stats, saved TokenSavingView, days int) {
	fmt.Fprintf(w, "%s · %d subagents · %s\n", headlineWindow(days), s.Total, headlineTokens(saved.Total))
	fmt.Fprintf(w, "Guardrails held %d risky moves · frontier rewrites on doubt: %d\n", s.Corrected, s.FrontierOnDoubt)
	fmt.Fprintf(w, "%s\n", headlineMix(s))
	if types := agentTypeLines(saved); len(types) > 1 || len(types) == 1 && types[0].AgentType != UntypedAgent {
		lines := make([]string, len(types))
		for i, t := range types {
			lines[i] = t.Line
		}
		fmt.Fprintf(w, "By agent type: %s\n", strings.Join(lines, " · "))
	}
}

func headlineWindow(days int) string {
	switch {
	case days <= 0:
		return "All time"
	case days == 1:
		return "Last 1 day"
	default:
		return fmt.Sprintf("Last %d days", days)
	}
}

func headlineTokens(t TokenSaving) string {
	if line := t.Line(); line != "" {
		return line
	}
	return "tokens not observed yet: wire SubagentStop with downshift setup"
}

// headlineMix renders the models children ended up on, most used first.
func headlineMix(s Stats) string {
	if len(s.ModelMix) == 0 {
		return "Models: none recorded"
	}
	type kv struct {
		model string
		n     int
	}
	rows := make([]kv, 0, len(s.ModelMix))
	for m, n := range s.ModelMix {
		rows = append(rows, kv{m, n})
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].n != rows[j].n {
			return rows[i].n > rows[j].n
		}
		return rows[i].model < rows[j].model
	})
	other := 0
	if len(rows) > maxMixModels {
		for _, r := range rows[maxMixModels:] {
			other += r.n
		}
		rows = rows[:maxMixModels]
	}
	parts := make([]string, 0, len(rows)+1)
	for _, r := range rows {
		parts = append(parts, fmt.Sprintf("%s %d", r.model, r.n))
	}
	if other > 0 {
		parts = append(parts, fmt.Sprintf("other %d", other))
	}
	return "Models: " + strings.Join(parts, " · ")
}
