// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package telemetry

import (
	"fmt"
	"io"
	"sort"
)

// CostGroup is the observed, catalog-priced cost of one session or one subagent.
// It is an estimate from recorded usage, not a bill.
type CostGroup struct {
	Events int
	USD    float64
}

// addGroupCost folds one priced usage event into a group map. An empty key is
// skipped: a record with no session or agent id cannot be attributed.
func addGroupCost(m map[string]CostGroup, key string, usd float64) {
	if key == "" {
		return
	}
	g := m[key]
	g.Events++
	g.USD += usd
	m[key] = g
}

// groupStats returns the number of groups, the mean and the maximum cost.
func groupStats(m map[string]CostGroup) (n int, mean, max float64) {
	if len(m) == 0 {
		return 0, 0, 0
	}
	var sum float64
	for _, g := range m {
		sum += g.USD
		if g.USD > max {
			max = g.USD
		}
	}
	return len(m), sum / float64(len(m)), max
}

// topGroups returns up to limit groups ordered by cost, then key, so output is
// deterministic.
func topGroups(m map[string]CostGroup, limit int) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if m[keys[i]].USD != m[keys[j]].USD {
			return m[keys[i]].USD > m[keys[j]].USD
		}
		return keys[i] < keys[j]
	})
	if len(keys) > limit {
		keys = keys[:limit]
	}
	return keys
}

// printCostGroups writes observed cost per session and per subagent. It prints
// nothing when no priced usage could be attributed, so output without usage is
// unchanged. Ids are the hashed ids already stored; only a short prefix is shown.
func printCostGroups(w io.Writer, sessions, subagents map[string]CostGroup) {
	if len(sessions) == 0 && len(subagents) == 0 {
		return
	}
	if n, mean, max := groupStats(sessions); n > 0 {
		fmt.Fprintf(w, "  Sessions with cost   %6d  (mean $%.4f, max $%.4f)\n", n, mean, max)
		for _, k := range topGroups(sessions, 3) {
			fmt.Fprintf(w, "    session %-8.8s $%.4f  (%d usage records)\n", k, sessions[k].USD, sessions[k].Events)
		}
	}
	if n, mean, max := groupStats(subagents); n > 0 {
		fmt.Fprintf(w, "  Subagents with cost  %6d  (mean $%.4f, max $%.4f)\n", n, mean, max)
	}
	fmt.Fprintf(w, "  Cost per session and subagent is catalog-priced from recorded usage; subagents counted only when the record carries an agent id.\n")
}
