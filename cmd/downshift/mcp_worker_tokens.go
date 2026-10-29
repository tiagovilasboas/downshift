// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/tiagovilasboas/downshift/internal/telemetry"
	"github.com/tiagovilasboas/downshift/internal/workers"
)

// workerHarness names the harness whose CLI a worker agent runs, for telemetry.
var workerHarness = map[string]string{"claude": "claude-code", "codex": "codex"}

// wireWorkerTokens sets the per-CLI token budget from the environment and
// records one telemetry event per finished worker. Tokens only, no dollars.
func wireWorkerTokens(q *workers.Queue, getenv func(string) string, errW io.Writer) {
	budget, err := workerTokenBudget(getenv(workers.TokenBudgetEnv))
	if err != nil {
		fmt.Fprintf(errW, "mcp serve: %s ignored: %v\n", workers.TokenBudgetEnv, err)
	}
	q.SetTokenBudget(budget)
	q.OnEnd(func(e workers.End) { telemetry.Record(workerEndEvent(e)) })
}

// workerTokenBudget reads the budget: empty or "0" is off, anything else must be
// a positive whole number of tokens.
func workerTokenBudget(raw string) (int64, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, nil
	}
	n, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || n < 0 {
		return 0, fmt.Errorf("%q is not a whole number of tokens", raw)
	}
	return n, nil
}

// workerEndEvent maps a worker's end onto a telemetry record. Cached tokens in
// telemetry are cache reads plus cache writes, with the writes also kept apart.
func workerEndEvent(e workers.End) telemetry.Event {
	harness := workerHarness[e.Agent]
	if harness == "" {
		harness = "unknown"
	}
	var usage *telemetry.TokenUsage
	if t := e.Tokens; t != nil {
		usage = &telemetry.TokenUsage{
			InputTokens:      t.Input,
			OutputTokens:     t.Output,
			CachedTokens:     t.CacheRead + t.CacheWrite,
			CacheWriteTokens: t.CacheWrite,
		}
	}
	return telemetry.WorkerEnded(harness, string(e.Outcome), usage)
}
