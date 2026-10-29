// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package accounting

import (
	"fmt"
	"sort"
	"strings"

	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

// WorkerTokens is what the MCP delegate workers of one CLI spent over a
// Scope, from their worker_ended events. Tokens only, never money.
//
// These tokens are ADDITIONAL to Ledger.Spent, not part of it: Spent counts
// the usage events of routed subagent spawns, and the worker_ended events are
// a separate record that telemetry.TokenSavings and Aggregate both skip, so a
// worker is never counted in both. They are shown on their own line.
type WorkerTokens struct {
	// Agent is the worker CLI: "claude" or "codex".
	Agent      string `json:"agent"`
	Workers    int64  `json:"workers"`
	Input      int64  `json:"input"`
	Output     int64  `json:"output"`
	CacheRead  int64  `json:"cache_read"`
	CacheWrite int64  `json:"cache_write"`
	// Total is input + output + cache read + cache write.
	Total int64 `json:"total"`
}

// workerAgent maps the harness a worker_ended event names back to its CLI.
// It is the inverse of cmd/downshift's workerHarness; an unknown harness
// keeps its own name so no worker tokens are dropped.
var workerAgent = map[string]string{"claude-code": "claude", "codex": "codex"}

// workerTokensOf sums the worker_ended events per CLI, ordered by agent name.
// A worker that reported no tokens (killed by its time budget) still counts
// in Workers. Events carry CachedTokens = read + write, so the read part is
// the difference.
func workerTokensOf(events []telemetry.Event) []WorkerTokens {
	by := map[string]*WorkerTokens{}
	for _, ev := range events {
		if ev.Outcome != telemetry.OutcomeWorkerEnded {
			continue
		}
		agent := workerAgent[ev.Harness]
		if agent == "" {
			agent = ev.Harness
		}
		w := by[agent]
		if w == nil {
			w = &WorkerTokens{Agent: agent}
			by[agent] = w
		}
		w.Workers++
		if !ev.HasTokenUsage() {
			continue
		}
		u := ev.TokenUsageOrZero()
		w.Input += u.InputTokens
		w.Output += u.OutputTokens
		w.CacheWrite += u.CacheWriteTokens
		w.CacheRead += max(u.CachedTokens-u.CacheWriteTokens, 0)
	}
	var out []WorkerTokens
	for _, w := range by {
		w.Total = w.Input + w.Output + w.CacheRead + w.CacheWrite
		out = append(out, *w)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Agent < out[j].Agent })
	return out
}

// WorkersLine is the printed worker line, or "" when no worker ended in the
// window, e.g. "Workers (additional to spent): claude 12.0k · codex 3.1k".
func WorkersLine(ws []WorkerTokens) string {
	if len(ws) == 0 {
		return ""
	}
	parts := make([]string, 0, len(ws))
	for _, w := range ws {
		parts = append(parts, fmt.Sprintf("%s %s", w.Agent, telemetry.ShortTokens(w.Total)))
	}
	return "Workers (additional to spent): " + strings.Join(parts, " · ")
}
