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
// routing tokens per harness, the
// guardrails, the model mix and, when spawns name one, routing tokens per agent
// type. Tokens only.
func printHeadline(w io.Writer, s Stats, routing RoutingView, days int) {
	fmt.Fprintf(w, "%s · %d subagents\n", headlineWindow(days), s.Total)
	printHeadlineTokens(w, routing)
	fmt.Fprintf(w, "Guardrails held %d risky moves · frontier rewrites on doubt: %d\n", s.Corrected, s.FrontierOnDoubt)
	fmt.Fprintf(w, "%s\n", headlineMix(s))
	if lines := BelowTypeLines(AgentTypeLines(routing)); len(lines) > 0 {
		fmt.Fprintf(w, "By type: %s\n", strings.Join(lines, " · "))
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

// printHeadlineTokens writes one routing line per harness, never a total, or
// the setup hint when no tokens are observed.
func printHeadlineTokens(w io.Writer, v RoutingView) {
	order := v.HarnessOrder()
	if len(order) == 0 {
		fmt.Fprintln(w, "tokens not observed yet: wire SubagentStop with downshift setup")
		return
	}
	for _, name := range order {
		line := v.ByHarness[name].Line()
		if line == "" {
			line = "unobserved"
		}
		fmt.Fprintf(w, "%s: %s\n", name, line)
	}
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
