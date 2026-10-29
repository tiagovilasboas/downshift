// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"testing"

	"github.com/tiagovilasboas/downshift/internal/workers"
)

func TestWorkerTokenBudgetParsing(t *testing.T) {
	cases := []struct {
		raw     string
		want    int64
		wantErr bool
	}{
		{"", 0, false},
		{"0", 0, false},
		{" 1000000 ", 1_000_000, false},
		{"1e6", 0, true},
		{"-5", 0, true},
		{"lots", 0, true},
	}
	for _, c := range cases {
		got, err := workerTokenBudget(c.raw)
		if got != c.want || (err != nil) != c.wantErr {
			t.Errorf("workerTokenBudget(%q) = %d, %v", c.raw, got, err)
		}
	}
}

func TestWorkerEndEventMapsTheCLIAndItsTokens(t *testing.T) {
	ev := workerEndEvent(workers.End{Agent: "claude", Outcome: workers.Done, Tokens: &workers.Tokens{Input: 10, Output: 20, CacheRead: 300, CacheWrite: 40, Total: 370}})
	u := ev.TokenUsageOrZero()
	if ev.Harness != "claude-code" || ev.WorkerOutcome != "done" || u.InputTokens != 10 || u.OutputTokens != 20 || u.CachedTokens != 340 || u.CacheWriteTokens != 40 {
		t.Fatalf("event = %+v usage = %+v", ev, u)
	}
	if none := workerEndEvent(workers.End{Agent: "codex", Outcome: workers.TimedOut}); none.Harness != "codex" || none.HasTokenUsage() {
		t.Fatalf("event = %+v", none)
	}
}
