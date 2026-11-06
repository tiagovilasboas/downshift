// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package telemetry

import (
	"bytes"
	"strings"
	"testing"
)

// tokenSpawn adds token counts to the report fixture's usage records.
func tokenSpawn(f *reportFixture, group string, cost float64, in, out int64, status ChildStatus) string {
	id := f.spawn(group, cost, status, "s1")
	for i := range f.events {
		if f.events[i].Outcome == OutcomeUsage && f.events[i].LinkedDecision == id {
			f.events[i].InputTokens, f.events[i].OutputTokens = &in, &out
		}
	}
	return id
}

func TestBuildReport_TokensPerCompletedTask(t *testing.T) {
	f := newFixture()
	for i := 0; i < 25; i++ {
		tokenSpawn(f, GroupRouted, 0.1, 100, 200, ChildCompleted)
	}
	// Not completed, unpriced or unrouted spawns never enter the figure.
	tokenSpawn(f, GroupRouted, 0.1, 9999, 9999, ChildError)
	tokenSpawn(f, GroupRouted, -1, 9999, 9999, ChildCompleted)
	tokenSpawn(f, GroupUnrouted, 0.1, 9999, 9999, ChildCompleted)
	rep := BuildReport(f.events, 30)
	if rep.Routed.Completed != 25 || rep.Routed.Tokens.Total != 25*300 || rep.Routed.TokensPerTask != 300 {
		t.Errorf("routed = %d completed, %d tokens, %v per task; want 25, 7500, 300", rep.Routed.Completed, rep.Routed.Tokens.Total, rep.Routed.TokensPerTask)
	}
	out, err := ReportJSON(rep)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"tokens_per_completed_task": 300`, `"tokens": {`, `"billable_tokens_per_completed_task": 300`} {
		if !strings.Contains(out, want) {
			t.Errorf("report JSON is missing %s:\n%s", want, out)
		}
	}
	var b bytes.Buffer
	PrintReport(&b, rep)
	for _, want := range []string{"billable/completed", "300"} {
		if !strings.Contains(b.String(), want) {
			t.Errorf("report text is missing %q:\n%s", want, b.String())
		}
	}
}

// A completed task carries the tokens of the failed attempt it redid, the same
// way it carries its cost.
func TestBuildReport_TokensIncludeRework(t *testing.T) {
	f := newFixture()
	tokenSpawn(f, GroupRouted, 0.1, 10, 10, ChildError)
	tokenSpawn(f, GroupRouted, 0.1, 100, 100, ChildCompleted)
	rep := BuildReport(f.events, 30)
	if rep.Routed.Completed != 1 || rep.Routed.Tokens.Total != 220 || rep.Routed.TokensPerTask != 220 {
		t.Errorf("routed tokens = %d (%v per task) over %d completed, want 220 with the redone attempt", rep.Routed.Tokens.Total, rep.Routed.TokensPerTask, rep.Routed.Completed)
	}
}
