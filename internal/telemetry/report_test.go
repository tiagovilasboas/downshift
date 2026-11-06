// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package telemetry

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
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
	case GroupUnrouted:
		d.Outcome, d.ReasonCodes = OutcomeBaseline, []string{ReasonPaused}
	}
	f.events = append(f.events, d)
	if cost >= 0 {
		// Tokens scale with the cost: 1 dollar here is 10,000 input tokens,
		// so every figure the report prints is tokens.
		in := int64(math.Round(cost * 1e4))
		f.events = append(f.events, Event{
			CorrelationID: fmt.Sprintf("u%031x", f.n), Timestamp: f.at.Add(10 * time.Second).Format(time.RFC3339Nano),
			Harness: "claude-code", SessionID: session, Outcome: OutcomeUsage, LinkedDecision: id,
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

func TestBuildReport_RoutedRowAndDeterminism(t *testing.T) {
	f := newFixture()
	for i := 0; i < 4; i++ {
		f.spawn(GroupRouted, 0.10, ChildCompleted, "s1")
	}
	rep := BuildReport(f.events, 30)
	g := rep.Routed
	if g.Spawns != 4 || g.Ended != 4 || g.Completed != 4 || g.BillablePerTask != 1000 || g.Reworked != 0 {
		t.Fatalf("routed row = %+v, want 4 spawns, 4 ended, 4 completed, 1000 billable a task, no rework", g)
	}
	var b strings.Builder
	PrintReport(&b, rep)
	out := b.String()
	if !strings.Contains(out, "Routed spawns, last 30 days") || !strings.Contains(out, "routed") {
		t.Errorf("text report must lead with the routed row:\n%s", out)
	}
	for _, gone := range []string{"control", "Control", "not enough data", "interval", "worse"} {
		if strings.Contains(out, gone) {
			t.Errorf("routed-only report must not mention %q:\n%s", gone, out)
		}
	}
	again, _ := ReportJSON(BuildReport(f.events, 30))
	first, _ := ReportJSON(rep)
	if first != again {
		t.Error("the same log must produce the same report")
	}
}

func TestPrintReport_EmptyWindowSaysNoRoutedSpawn(t *testing.T) {
	var b strings.Builder
	PrintReport(&b, BuildReport(nil, 7))
	if !strings.Contains(b.String(), "No routed spawn in this window.") {
		t.Errorf("an empty window must say so:\n%s", b.String())
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

func TestBuildReport_ExcludesUnroutedAndUnpriced(t *testing.T) {
	f := newFixture()
	f.spawn(GroupUnrouted, 0.5, ChildCompleted, "s1") // paused
	f.spawn(GroupRouted, -1, ChildCompleted, "s1")    // no usage record
	f.spawn(GroupRouted, -1, ChildLaunched, "s1")     // launched, never stopped: no usage record
	rep := BuildReport(f.events, 30)
	if rep.Excluded != 1 {
		t.Errorf("excluded = %d, want 1", rep.Excluded)
	}
	if rep.Routed.Completed != 0 {
		t.Errorf("unpriced or non-completed tasks must not count: %+v", rep.Routed)
	}
	if rep.Routed.Spawns != 2 {
		t.Errorf("spawns = %d, want 2 (the paused spawn is not one)", rep.Routed.Spawns)
	}
}

// A log from an older binary holds `outcome: baseline` lines, some with the
// CONTROL reason code of the retired control group. They load without error,
// are counted as excluded, and move no routed figure.
func TestBuildReport_OldBaselineLinesAreReadAndExcluded(t *testing.T) {
	now := time.Now().UTC()
	line := func(id, extra string, minutes int) string {
		return `{"correlation_id":"` + id + `","timestamp":"` + now.Add(time.Duration(-minutes)*time.Minute).Format(time.RFC3339Nano) +
			`","harness":"claude-code","session_id":"s1","outcome":"baseline","verdict":"DOWNSHIFT",` +
			`"requested_model":"claude-opus-5-5","final_model":"claude-opus-5-5"` + extra + `}`
	}
	routed := `{"correlation_id":"r1","timestamp":"` + now.Add(-time.Minute).Format(time.RFC3339Nano) +
		`","harness":"claude-code","session_id":"s1","outcome":"rewrite_emitted","verdict":"DOWNSHIFT"}`
	path := filepath.Join(t.TempDir(), "events.jsonl")
	body := strings.Join([]string{
		line("b1", `,"reason_codes":["CONTROL"]`, 30), // the old control group
		line("b2", ``, 20), // the old no-route switch
		routed,
	}, "\n") + "\n"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	events, err := ReadEventsFrom(path)
	if err != nil || len(events) != 3 {
		t.Fatalf("an old baseline line must load: %d events, err %v", len(events), err)
	}
	rep := BuildReport(events, 30)
	if rep.Excluded != 2 {
		t.Errorf("excluded = %d, want the 2 old baseline lines", rep.Excluded)
	}
	if rep.Routed.Spawns != 1 {
		t.Errorf("routed spawns = %d, want 1: baseline lines are not routed", rep.Routed.Spawns)
	}
	var b strings.Builder
	PrintReport(&b, rep)
	if !strings.Contains(b.String(), "2 spawn(s) left unrouted by a session pause, or logged as baseline by an older version") {
		t.Errorf("the excluded line must say what was left out:\n%s", b.String())
	}
	for _, ev := range events[:2] {
		if SpawnStateOf(ev) == StatePaused {
			t.Errorf("an old baseline line is not a pause: %+v", ev)
		}
	}
}

// Claude Code background children only ever report "launched"; the usage
// record written at SubagentStop proves the child finished. A real failure end
// still wins over it.
func TestBuildTasks_UsageMarksBackgroundChildCompleted(t *testing.T) {
	dec := Event{Timestamp: "2026-10-13T10:00:00Z", Harness: "claude-code", SessionID: "s", Outcome: OutcomeRewriteEmitted, Verdict: "DOWNSHIFT", Tier: "small"}
	key := DecisionKey(dec)
	launched := Event{Timestamp: "2026-10-13T10:00:01Z", Harness: "claude-code", SessionID: "s", Outcome: OutcomeChildEnded, ChildStatus: ChildLaunched, LinkedDecision: key}
	in := int64(1000)
	usage := Event{Timestamp: "2026-10-13T10:01:00Z", Harness: "claude-code", SessionID: "s", Outcome: OutcomeUsage, LinkedDecision: key, InputTokens: &in}
	for _, order := range [][]Event{{dec, launched, usage}, {dec, usage, launched}} {
		tasks := BuildTasks(order)
		if len(tasks) != 1 || !tasks[0].Completed() || !tasks[0].Observed {
			t.Fatalf("background child with usage must be completed and priced: %+v", tasks)
		}
	}
	failed := Event{Timestamp: "2026-10-13T10:00:30Z", Harness: "claude-code", SessionID: "s", Outcome: OutcomeChildEnded, ChildStatus: ChildError, LinkedDecision: key}
	if tasks := BuildTasks([]Event{dec, failed, usage}); tasks[0].Completed() {
		t.Fatal("an error end must outrank the usage record")
	}
}

func TestPrintReport_RoutingIsPerHarnessWithNoTotal(t *testing.T) {
	rep := Report{Routing: RoutingView{ByHarness: map[string]RoutingRow{
		"claude-code": {Spawns: 3, Billable: 1000, CacheRead: 500},
		"codex":       {UnobservedSpawns: 1},
	}}}
	var b strings.Builder
	PrintReport(&b, rep)
	out := b.String()
	for _, want := range []string{"claude-code", "codex", "ran below the session model  0 of 1.0k billable (0%", "cache read  500", "unobserved  1 spawn "} {
		if !strings.Contains(out, want) {
			t.Errorf("report lacks %q:\n%s", want, out)
		}
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
	if _, ok := top["routing_tokens"]; !ok {
		t.Errorf("report JSON must carry routing_tokens")
	}
	for _, key := range []string{"tokens_saved", "effect"} {
		if _, ok := top[key]; ok {
			t.Errorf("report JSON must not carry the replaced %q", key)
		}
	}
}

// A failed routed attempt redone by an unrouted (paused) task keeps its rework
// flag in the routed group, but its tokens must not move to the unrouted task:
// routed and unrouted tokens are never mixed.
func TestBuildTasks_ReworkDoesNotCrossGroups(t *testing.T) {
	f := newFixture()
	failed := f.spawn(GroupRouted, 0.20, ChildError, "s1")
	redo := f.spawn(GroupUnrouted, 0.30, ChildCompleted, "s1")
	sameGroupFailed := f.spawn(GroupUnrouted, 0.10, ChildError, "s2")
	sameGroupRedo := f.spawn(GroupUnrouted, 0.40, ChildCompleted, "s2")

	byKey := map[string]TaskRecord{}
	for _, tr := range BuildTasks(f.events) {
		byKey[tr.Key] = tr
	}
	if !byKey[failed].Reworked {
		t.Error("the failed routed attempt is rework of the routed group")
	}
	if len(byKey[redo].Redo) != 0 || byKey[redo].TotalTokens.Total != 3000 {
		t.Errorf("unrouted redo = %+v, want its own 3000 tokens and no redo link", byKey[redo])
	}
	if len(byKey[sameGroupRedo].Redo) != 1 || byKey[sameGroupRedo].Redo[0] != sameGroupFailed || byKey[sameGroupRedo].TotalTokens.Total != 5000 {
		t.Errorf("a redo inside one group still carries the failed attempt: %+v", byKey[sameGroupRedo])
	}
}
