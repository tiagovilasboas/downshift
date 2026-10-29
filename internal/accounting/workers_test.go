// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package accounting

import (
	"reflect"
	"testing"
	"time"

	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

// worker is a worker_ended event; cached is cache read + write, as the emitter writes it.
func worker(harness string, when time.Duration, in, out, cached, write int64) telemetry.Event {
	return telemetry.Event{Timestamp: ago(when), Harness: harness, Outcome: telemetry.OutcomeWorkerEnded,
		InputTokens: i64(in), OutputTokens: i64(out), CachedTokens: i64(cached), CacheWriteTokens: i64(write)}
}

func TestWorkerTokensOf(t *testing.T) {
	noUsage := telemetry.Event{Timestamp: ago(time.Hour), Harness: "codex", Outcome: telemetry.OutcomeWorkerEnded}
	tests := []struct {
		name   string
		events []telemetry.Event
		want   []WorkerTokens
	}{
		{name: "none"},
		{name: "ignores other outcomes", events: cheaperChild("s", "a", time.Hour, 10, 5)},
		{name: "sums per CLI and splits cache read from write",
			events: []telemetry.Event{
				worker("claude-code", time.Hour, 100, 50, 30, 10),
				worker("claude-code", time.Hour, 1, 2, 3, 3),
				worker("codex", time.Hour, 7, 8, 0, 0),
			},
			want: []WorkerTokens{
				{Agent: "claude", Workers: 2, Input: 101, Output: 52, CacheRead: 20, CacheWrite: 13, Total: 186},
				{Agent: "codex", Workers: 1, Input: 7, Output: 8, Total: 15},
			}},
		{name: "a worker with no usage counts but adds no tokens",
			events: []telemetry.Event{noUsage},
			want:   []WorkerTokens{{Agent: "codex", Workers: 1}}},
		{name: "unknown harness keeps its name",
			events: []telemetry.Event{worker("other", time.Hour, 4, 0, 0, 0)},
			want:   []WorkerTokens{{Agent: "other", Workers: 1, Input: 4, Total: 4}}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := workerTokensOf(tc.events); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("workers = %+v, want %+v", got, tc.want)
			}
		})
	}
}

// Workers are additional to Spent and follow the ledger window.
func TestCompute_LedgerWorkers(t *testing.T) {
	events := append(cheaperChild("s1", "new", time.Hour, 100, 1000),
		worker("claude-code", time.Hour, 10, 20, 0, 0),
		worker("claude-code", 40*24*time.Hour, 500, 0, 0, 0))
	l := Compute(events, nil, Scope{Kind: ScopeDays, Days: 7}).Ledger
	if l.Spent != 1100 {
		t.Errorf("spent = %d, want 1100 (workers are not in it)", l.Spent)
	}
	want := []WorkerTokens{{Agent: "claude", Workers: 1, Input: 10, Output: 20, Total: 30}}
	if !reflect.DeepEqual(l.Workers, want) {
		t.Errorf("workers = %+v, want %+v", l.Workers, want)
	}
	if got := WorkersLine(l.Workers); got != "Workers (additional to spent): claude 30" {
		t.Errorf("line = %q", got)
	}
	if WorkersLine(nil) != "" {
		t.Error("no workers must print no line")
	}
}
