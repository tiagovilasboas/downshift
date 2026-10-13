// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
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

// printHeadline writes the three lines every `downshift stats` starts with:
// the money, the guardrails, and the model mix. The money comes only from
// events with real provider cost; with none, it says how to get some instead
// of showing a normalised proxy as if it were dollars.
func printHeadline(w io.Writer, s Stats, days int) {
	fmt.Fprintf(w, "%s · %d subagents · %s\n", headlineWindow(days), s.Total, headlineMoney(s))
	fmt.Fprintf(w, "Guardrails held %d risky moves · frontier rewrites on doubt: %d\n", s.Corrected, s.FrontierOnDoubt)
	fmt.Fprintf(w, "%s\n", headlineMix(s))
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

func headlineMoney(s Stats) string {
	if s.RealCostEvents == 0 {
		return "no real cost yet: wire SubagentStop with downshift setup"
	}
	net := s.RealCostDeltaUSD
	if net < 0 {
		return fmt.Sprintf("net cost $%.2f more (saved $%.2f, cost more $%.2f)", -net, s.RealSavedUSD, s.RealOverspendUSD)
	}
	return fmt.Sprintf("net saved $%.2f (saved $%.2f, cost more $%.2f)", net, s.RealSavedUSD, s.RealOverspendUSD)
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
