// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package telemetry

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
	"time"
)

// spawnTokens adds one completed task with every token class set, so a test
// can make cache reads large and noisy while the billable part stays fixed.
// lower makes the decision an applied move from a frontier to a small model,
// so the child's tokens ran on a lower tier than requested.
func (f *reportFixture) spawnTokens(group string, in, out, read, write int64, lower bool) {
	f.n++
	f.at = f.at.Add(time.Minute)
	id := fmt.Sprintf("%032x", f.n)
	d := Event{CorrelationID: id, Timestamp: f.at.Format(time.RFC3339Nano), Harness: "claude-code",
		SessionID: "s1", Outcome: OutcomeRewriteEmitted, Verdict: "DOWNSHIFT",
		FromModel: "claude-opus-5-5", ToModel: "claude-opus-5-5"}
	if lower {
		d.ToModel = "claude-haiku-4-5"
	}
	if group == GroupUnrouted {
		d.Outcome, d.ReasonCodes = OutcomeBaseline, []string{ReasonPaused}
	}
	at := func(s int) string { return f.at.Add(time.Duration(s) * time.Second).Format(time.RFC3339Nano) }
	f.events = append(f.events, d,
		Event{CorrelationID: fmt.Sprintf("u%031x", f.n), Timestamp: at(10), Harness: "claude-code", SessionID: "s1",
			Outcome: OutcomeUsage, LinkedDecision: id,
			InputTokens: i64(in), OutputTokens: i64(out), CachedTokens: i64(read + write), CacheWriteTokens: i64(write)},
		Event{CorrelationID: fmt.Sprintf("e%031x", f.n), Timestamp: at(20), Harness: "claude-code", SessionID: "s1",
			Outcome: OutcomeChildEnded, LinkedDecision: id, ChildStatus: ChildCompleted})
}

func TestTokenClassesBillableLeavesOutCacheReads(t *testing.T) {
	c := TokenClasses{Input: 1, Output: 2, CacheRead: 4, CacheWrite: 8, Total: 15}
	if got := c.Billable(); got != 11 {
		t.Errorf("billable = %d, want 11 (input + output + cache write)", got)
	}
}

// Billable tokens per task leave the cache reads out: routed tasks with the
// same input, output and cache write have the same figure however large their
// cache reads are, while the total with cache reads follows the rereads.
func TestBuildReport_BillableLeavesOutCacheReads(t *testing.T) {
	f := newFixture()
	for i := 0; i < 30; i++ {
		f.spawnTokens(GroupRouted, 1000, 500, 900_000+int64(i%5)*100_000, 200, false)
	}
	rep := BuildReport(f.events, 30)
	if rep.Routed.BillablePerTask != 1700 {
		t.Fatalf("billable per task = %v, want 1700 whatever the cache reads", rep.Routed.BillablePerTask)
	}
	if rep.Routed.TokensPerTask < 500_000 {
		t.Errorf("all tokens per task keep the cache reads: %v", rep.Routed.TokensPerTask)
	}
	var b bytes.Buffer
	PrintReport(&b, rep)
	for _, want := range []string{"billable/completed", "1.7k"} {
		if !strings.Contains(b.String(), want) {
			t.Errorf("report text is missing %q:\n%s", want, b.String())
		}
	}
}

// --- unhappy paths ---

// Paused tasks are not routed: their tokens enter no routed figure.
func TestBuildReport_UnroutedTokensStayOutOfTheRoutedRow(t *testing.T) {
	f := newFixture()
	f.spawnTokens(GroupRouted, 1000, 500, 5000, 0, false)
	f.spawnTokens(GroupUnrouted, 900_000, 100_000, 0, 0, false)
	rep := BuildReport(f.events, 30)
	if rep.Routed.Completed != 1 || rep.Routed.BillablePerTask != 1500 {
		t.Fatalf("routed row = %+v, want the one routed task at 1500 billable", rep.Routed)
	}
	if rep.Excluded != 1 {
		t.Errorf("excluded = %d, want the paused task", rep.Excluded)
	}
}
