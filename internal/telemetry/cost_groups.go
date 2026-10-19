// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package telemetry

import (
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
