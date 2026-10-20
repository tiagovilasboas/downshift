// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package telemetry

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"testing"
	"time"
)

// reportFixture builds a synthetic log. Each spawn is a decision, a usage
// record and a child_ended record, spaced one minute apart in one session.
type reportFixture struct {
	events []Event
	at     time.Time
	n      int
}

func newFixture() *reportFixture {
	return &reportFixture{at: time.Now().UTC().Add(-time.Hour)}
}

func (f *reportFixture) spawn(group string, cost float64, status ChildStatus, session string) string {
	f.n++
	f.at = f.at.Add(time.Minute)
	id := fmt.Sprintf("%032x", f.n)
	d := Event{
		CorrelationID: id, Timestamp: f.at.Format(time.RFC3339Nano), Harness: "claude-code",
		SessionID: session, Outcome: OutcomeRewriteEmitted,
	}
	switch group {
	case GroupControl:
		d.Outcome, d.ReasonCodes = OutcomeBaseline, []string{ReasonControl}
	case GroupOther:
		d.Outcome = OutcomeBaseline
	}
	f.events = append(f.events, d)
	if cost >= 0 {
		c := cost
		// Tokens scale with the cost: 1 dollar here is 10,000 input tokens,
		// so every figure the report prints is tokens.
		in := int64(math.Round(cost * 1e4))
		f.events = append(f.events, Event{
			CorrelationID: fmt.Sprintf("u%031x", f.n), Timestamp: f.at.Add(10 * time.Second).Format(time.RFC3339Nano),
			Harness: "claude-code", SessionID: session, Outcome: OutcomeUsage, LinkedDecision: id, ActualCostUSD: &c,
			InputTokens: &in,
		})
	}
	if status != "" {
		f.events = append(f.events, Event{
			CorrelationID: fmt.Sprintf("e%031x", f.n), Timestamp: f.at.Add(20 * time.Second).Format(time.RFC3339Nano),
			Harness: "claude-code", SessionID: session, Outcome: OutcomeChildEnded, LinkedDecision: id, ChildStatus: status,
		})
	}
	return id
}

func TestBuildReport_NotEnoughData(t *testing.T) {
	f := newFixture()
	for i := 0; i < 19; i++ {
		f.spawn(GroupRouted, 0.1, ChildCompleted, "s1")
		f.spawn(GroupControl, 0.2, ChildCompleted, "s1")
	}
	rep := BuildReport(f.events, 30)
	if rep.Enough || rep.Savings != nil {
		t.Fatalf("19 tasks per group must not produce savings: %+v", rep)
	}
	if !strings.Contains(rep.Message, "not enough data") {
		t.Errorf("message = %q", rep.Message)
	}
	var b strings.Builder
	PrintReport(&b, rep)
	if !strings.Contains(b.String(), "not enough data") || strings.Contains(b.String(), "Savings per") {
		t.Errorf("text report must say not enough data and print no savings:\n%s", b.String())
	}
}

func TestBuildReport_SavingsAndDeterministicCI(t *testing.T) {
	f := newFixture()
	for i := 0; i < 40; i++ {
		f.spawn(GroupRouted, 0.05+0.01*float64(i%5), ChildCompleted, "s1")
		f.spawn(GroupControl, 0.10+0.02*float64(i%5), ChildCompleted, "s1")
	}
	rep := BuildReport(f.events, 30)
	if !rep.Enough || rep.Savings == nil {
		t.Fatalf("expected savings: %+v", rep)
	}
	s := rep.Savings
	// control averages 1,400 tokens a task, routed 700: about 700 saved per task.
	if s.PerTaskTokens < 650 || s.PerTaskTokens > 750 {
		t.Errorf("savings per task = %.1f tokens, want about 700", s.PerTaskTokens)
	}
	if !(s.PerTaskTokensLow <= s.PerTaskTokens && s.PerTaskTokens <= s.PerTaskTokensHigh) || s.PerTaskTokensLow <= 0 {
		t.Errorf("CI must bracket a positive estimate: %+v", s)
	}
	if raw, _ := ReportJSON(rep); strings.Contains(raw, "percent") {
		t.Errorf("the report states no dollar percentage: %s", raw)
	}
	again := BuildReport(f.events, 30)
	a, _ := ReportJSON(rep)
	b, _ := ReportJSON(again)
	if a != b {
		t.Error("the same log must produce the same report, interval included")
	}
	if rep.Routed.Completed != 40 || rep.Control.Completed != 40 {
		t.Errorf("completed = %d/%d, want 40/40", rep.Routed.Completed, rep.Control.Completed)
	}
}

func TestBuildReport_NoSavingsIntervalIncludesZero(t *testing.T) {
	f := newFixture()
	for i := 0; i < 30; i++ {
		c := 0.10 + 0.05*float64(i%4)
		f.spawn(GroupRouted, c, ChildCompleted, "s1")
		f.spawn(GroupControl, c, ChildCompleted, "s1")
	}
	rep := BuildReport(f.events, 30)
	if rep.Savings == nil || rep.Savings.PerTaskTokensLow > 0 || rep.Savings.PerTaskTokensHigh < 0 {
		t.Fatalf("identical groups must give an interval around zero: %+v", rep.Savings)
	}
}

func TestBuildTasks_ReworkLinkingAndCost(t *testing.T) {
	f := newFixture()
	failed := f.spawn(GroupRouted, 0.30, ChildError, "s1")
	redo := f.spawn(GroupRouted, 0.20, ChildCompleted, "s1")
	late := f.spawn(GroupRouted, 0.05, ChildCompleted, "s1")
	f.at = f.at.Add(30 * time.Minute) // outside the 10 min window
	lone := f.spawn(GroupRouted, 0.07, ChildError, "s2")
	after := f.spawn(GroupRouted, 0.09, ChildCompleted, "s2")
	_ = late

	byKey := map[string]TaskRecord{}
	for _, tk := range BuildTasks(f.events) {
		byKey[tk.Key] = tk
	}
	if !byKey[failed].Reworked || !byKey[failed].Failed() {
		t.Errorf("failed spawn must be marked reworked: %+v", byKey[failed])
	}
	if got := byKey[redo].TotalTokens.Total; got != 5000 {
		t.Errorf("completed redo total = %d tokens, want 5000 (0.20 + 0.30 rework at 10,000 tokens a dollar)", got)
	}
	if len(byKey[redo].Redo) != 1 || byKey[redo].Redo[0] != failed {
		t.Errorf("redo links = %v, want [%s]", byKey[redo].Redo, failed)
	}
	if byKey[lone].Reworked == false {
		// s2 failure is followed within 10 min by `after`, so it is reworked.
		t.Error("s2 failure followed by a respawn inside the window must be reworked")
	}
	if got := byKey[after].TotalTokens.Total; got != 1600 {
		t.Errorf("after total = %d tokens, want 1600", got)
	}

	rep := BuildReport(f.events, 30)
	if rep.Routed.Reworked != 2 || rep.Routed.Ended != 5 {
		t.Errorf("reworked/ended = %d/%d, want 2/5", rep.Routed.Reworked, rep.Routed.Ended)
	}
	if rep.Routed.Completed != 3 {
		t.Errorf("completed = %d, want 3", rep.Routed.Completed)
	}
	// 0.50 + 0.05 + 0.16 dollars at 10,000 tokens each.
	if want := int64(7100); rep.Routed.Tokens.Total != want {
		t.Errorf("tokens = %d, want %d", rep.Routed.Tokens.Total, want)
	}
}

func TestBuildReport_ExcludesNoRouteAndUnpriced(t *testing.T) {
	f := newFixture()
	f.spawn(GroupOther, 0.5, ChildCompleted, "s1") // NO_ROUTE / paused
	f.spawn(GroupRouted, -1, ChildCompleted, "s1") // no usage record
	f.spawn(GroupControl, -1, ChildLaunched, "s1") // launched, never stopped: no usage record
	rep := BuildReport(f.events, 30)
	if rep.Excluded != 1 {
		t.Errorf("excluded = %d, want 1", rep.Excluded)
	}
	if rep.Routed.Completed != 0 || rep.Control.Completed != 0 {
		t.Errorf("unpriced or non-completed tasks must not count: %+v %+v", rep.Routed, rep.Control)
	}
	if rep.Routed.Spawns != 1 || rep.Control.Spawns != 1 {
		t.Errorf("spawns = %d/%d, want 1/1", rep.Routed.Spawns, rep.Control.Spawns)
	}
}

// Claude Code background children only ever report "launched"; the usage
// record written at SubagentStop proves the child finished. A real failure end
// still wins over it.
func TestBuildTasks_UsageMarksBackgroundChildCompleted(t *testing.T) {
	dec := Event{Timestamp: "2026-10-13T10:00:00Z", Harness: "claude-code", SessionID: "s", Outcome: OutcomeRewriteEmitted, Verdict: "DOWNSHIFT", Tier: "small"}
	key := DecisionKey(dec)
	cost := 0.02
	launched := Event{Timestamp: "2026-10-13T10:00:01Z", Harness: "claude-code", SessionID: "s", Outcome: OutcomeChildEnded, ChildStatus: ChildLaunched, LinkedDecision: key}
	in := int64(1000)
	usage := Event{Timestamp: "2026-10-13T10:01:00Z", Harness: "claude-code", SessionID: "s", Outcome: OutcomeUsage, ActualCostUSD: &cost, LinkedDecision: key, InputTokens: &in}
	for _, order := range [][]Event{{dec, launched, usage}, {dec, usage, launched}} {
		tasks := BuildTasks(order)
		if len(tasks) != 1 || !tasks[0].Completed() || !tasks[0].Priced {
			t.Fatalf("background child with usage must be completed and priced: %+v", tasks)
		}
	}
	failed := Event{Timestamp: "2026-10-13T10:00:30Z", Harness: "claude-code", SessionID: "s", Outcome: OutcomeChildEnded, ChildStatus: ChildError, LinkedDecision: key}
	if tasks := BuildTasks([]Event{dec, failed, usage}); tasks[0].Completed() {
		t.Fatal("an error end must outrank the usage record")
	}
}

func TestPrintReport_NoTokensSavedSaysSo(t *testing.T) {
	rep := Report{TokensSaved: TokenSaving{Children: 3, WorkTokens: 1000, CacheTokens: 500}}
	var b strings.Builder
	PrintReport(&b, rep)
	if !strings.Contains(b.String(), "no tokens ran on a cheaper model in this window") {
		t.Errorf("a window with observed tokens and none saved must say so:\n%s", b.String())
	}
}

// The JSON report is tokens only: no dollar key at the top level, and the saved
// quantity is there.
func TestReportJSON_TokensOnly(t *testing.T) {
	out, err := ReportJSON(BuildReport(nil, 30))
	if err != nil {
		t.Fatal(err)
	}
	var top map[string]json.RawMessage
	if err := json.Unmarshal([]byte(out), &top); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"savings", "prices_as_of"} {
		if _, ok := top[key]; ok {
			t.Errorf("report JSON must not carry top-level %q", key)
		}
	}
	if _, ok := top["tokens_saved"]; !ok {
		t.Errorf("report JSON must carry tokens_saved")
	}
}
