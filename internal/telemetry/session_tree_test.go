// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package telemetry

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/tiagovilasboas/downshift/internal/core"
)

var sessA = strings.Repeat("a", 64)
var sessB = strings.Repeat("b", 64)

func td(id, sess string, d time.Duration, model string, codes ...string) Event {
	return Event{
		CorrelationID: id, Timestamp: at(d), SessionID: sess, Harness: "claude-code",
		Outcome: OutcomeRewriteEmitted, Complexity: "MEDIUM", FromModel: "claude-opus-5-5", ToModel: model,
		Tier: "mid", Verdict: "DOWNSHIFT", ReasonCodes: codes,
	}
}

func tuse(link, sess string, d time.Duration, model string, cost, base float64) Event {
	return Event{
		CorrelationID: "u-" + link, Timestamp: at(d), SessionID: sess, Harness: "claude-code", Outcome: OutcomeUsage,
		LinkedDecision: link, ToModel: model, ActualCostUSD: f64(cost), BaselineCostUSD: f64(base),
	}
}

func tend(link, sess string, st ChildStatus, d time.Duration) Event {
	e := childEnd(link, sess, st, d)
	e.Harness = "claude-code"
	return e
}

func lines(s string) []string { return strings.Split(strings.TrimRight(s, "\n"), "\n") }

func TestSessionTree_EmptyAndUnknownSession(t *testing.T) {
	if got := SessionTree(nil, sessA, TreeOptions{}); got != "session · no subagents yet\n" {
		t.Fatalf("empty: %q", got)
	}
	ev := []Event{td("d1", sessB, 0, "claude-haiku-5-5")}
	if got := SessionTree(ev, sessA, TreeOptions{}); !strings.Contains(got, "no subagents yet") {
		t.Fatalf("another session leaked into the view: %q", got)
	}
	if got := SessionTree(ev, "", TreeOptions{}); !strings.Contains(got, "no subagents yet") {
		t.Fatalf("empty session id matched events: %q", got)
	}
}

func TestSessionTree_OneSubagentWithRealCost(t *testing.T) {
	ev := []Event{
		td("d1", sessA, 0, "claude-sonnet-5-5", ReasonClassifierConfident),
		tuse("d1", sessA, time.Minute, "claude-sonnet-5-5", 0.22, 0.60),
	}
	got := lines(SessionTree(ev, sessA, TreeOptions{}))
	if got[0] != "session · 1 subagent · $0.22 (without downshift: $0.60) · saved $0.38" {
		t.Fatalf("header: %q", got[0])
	}
	if got[1] != "└─ subagent 1  sonnet  ✓  $0.22  MEDIUM, confident" {
		t.Fatalf("row: %q", got[1])
	}
}

func TestSessionTree_LossIsShownAsOver(t *testing.T) {
	ev := []Event{
		td("d1", sessA, 0, "claude-opus-5-5"),
		tuse("d1", sessA, time.Minute, "claude-opus-5-5", 1.30, 0.50),
	}
	out := SessionTree(ev, sessA, TreeOptions{})
	if !strings.Contains(out, "over $0.80") {
		t.Fatalf("loss not shown as over:\n%s", out)
	}
	hdr := lines(out)[0]
	if !strings.HasSuffix(hdr, "· over $0.80") || strings.Contains(hdr, "saved") {
		t.Fatalf("header: %q", hdr)
	}
	if strings.Contains(out, "-$") || strings.Contains(out, "saved -") {
		t.Fatalf("a negative amount leaked:\n%s", out)
	}
}

func TestSessionTree_StatusRunningErrorDone(t *testing.T) {
	ev := []Event{
		td("d1", sessA, 0, "claude-haiku-5-5"), // no end: running
		td("d2", sessA, time.Minute, "claude-haiku-5-5"),
		tend("d2", sessA, ChildError, 2*time.Minute), // failed, nothing after it
		td("d3", sessA, 3*time.Hour, "claude-haiku-5-5"),
		tend("d3", sessA, ChildCompleted, 3*time.Hour+time.Minute),
	}
	got := lines(SessionTree(ev, sessA, TreeOptions{Now: evTime(td("x", sessA, 3*time.Hour+2*time.Minute, ""))}))
	wantMarks := []string{markUnknown, markError, markDone}
	for i, m := range wantMarks {
		if !strings.Contains(got[i+1], "  "+m+"  ") {
			t.Errorf("row %d want mark %s: %q", i+1, m, got[i+1])
		}
	}
	if strings.Contains(strings.Join(got, "\n"), markRedone) {
		t.Fatalf("no respawn within the window, nothing is redone:\n%s", strings.Join(got, "\n"))
	}
}

func TestSessionTree_RunningTimesOutToUnknown(t *testing.T) {
	ev := []Event{td("d1", sessA, 0, "claude-haiku-5-5")}
	at := func(d time.Duration) TreeOptions { return TreeOptions{Now: evTime(ev[0]).Add(d)} }

	fresh := BuildSessionTree(ev, sessA, at(RunningTimeout))
	if r := fresh.Rows[0]; r.Status != "running" || r.mark != markRunning {
		t.Fatalf("exactly at the timeout it still runs: %+v", r)
	}
	stale := BuildSessionTree(ev, sessA, at(RunningTimeout+time.Second))
	if r := stale.Rows[0]; r.Status != "unknown" || r.mark != markUnknown {
		t.Fatalf("past the timeout the end is unknown: %+v", r)
	}
	if out := stale.Render(); !strings.Contains(out, "  ?  ") {
		t.Fatalf("render lacks ?:\n%s", out)
	}
	// An end record after the timeout still wins: the child did finish.
	ev = append(ev, tend("d1", sessA, ChildCompleted, 5*time.Hour))
	if r := BuildSessionTree(ev, sessA, at(6*time.Hour)).Rows[0]; r.Status != "done" {
		t.Fatalf("a recorded end must beat the timeout: %+v", r)
	}
}

func TestSessionTree_RedoIsNestedAndMarked(t *testing.T) {
	ev := []Event{
		td("d1", sessA, 0, "claude-haiku-5-5"),
		tend("d1", sessA, ChildError, time.Minute),
		td("d2", sessA, 2*time.Minute, "claude-sonnet-5-5"),
		tuse("d2", sessA, 4*time.Minute, "claude-sonnet-5-5", 0.10, 0.10),
		td("d3", sessA, 30*time.Minute, "claude-sonnet-5-5"),
		tuse("d3", sessA, 31*time.Minute, "claude-sonnet-5-5", 0.05, 0.05),
	}
	view := BuildSessionTree(ev, sessA, TreeOptions{})
	out := view.Render()
	got := lines(out)
	if len(got) != 4 {
		t.Fatalf("want header + 3 rows:\n%s", out)
	}
	if !strings.HasPrefix(got[1], "├─ subagent 1") || !strings.Contains(got[1], "  "+markRedone+"  ") {
		t.Errorf("failed child must be first and marked redone: %q", got[1])
	}
	if !strings.HasPrefix(got[2], "│  └─ subagent 2") || !strings.Contains(got[2], "  "+markDone+"  ") {
		t.Errorf("redo must nest under the failed child: %q", got[2])
	}
	if !strings.HasPrefix(got[3], "└─ subagent 3") {
		t.Errorf("later unrelated spawn is a root: %q", got[3])
	}
	if view.Rows[1].RedoOf != 1 || view.Rows[1].Depth != 1 || view.Rows[2].Depth != 0 {
		t.Errorf("json structure: %+v", view.Rows)
	}
}

func TestSessionTree_ColumnsAreAligned(t *testing.T) {
	ev := []Event{
		td("d1", sessA, 0, "claude-haiku-5-5", ReasonClassifierConfident),
		tuse("d1", sessA, time.Second, "claude-haiku-5-5", 0.01, 0.30),
		td("d2", sessA, time.Minute, "claude-sonnet-5-5"),
		tuse("d2", sessA, 2*time.Minute, "claude-sonnet-5-5", 12.5, 10),
		td("d3", sessA, 3*time.Minute, "claude-opus-5-5"),
	}
	var marks, costEnds []int
	for _, l := range lines(SessionTree(ev, sessA, TreeOptions{}))[1:] {
		r := []rune(l)
		for i, c := range r {
			if c == []rune(markDone)[0] || c == []rune(markRunning)[0] {
				marks = append(marks, i)
				break
			}
		}
		costEnds = append(costEnds, utf8.RuneCountInString(strings.TrimRight(l[:strings.Index(l, "  ")+0], " ")))
	}
	for _, m := range marks {
		if m != marks[0] {
			t.Fatalf("status column moves: %v", marks)
		}
	}
	// The cost column is right-aligned: its last digit sits at one column.
	var ends []int
	for _, l := range lines(SessionTree(ev, sessA, TreeOptions{}))[1:] {
		r := []rune(l)
		i := marks[0] + 3
		for i < len(r) && r[i] == ' ' {
			i++
		}
		for i < len(r) && r[i] != ' ' {
			i++
		}
		ends = append(ends, i)
	}
	for _, e := range ends {
		if e != ends[0] {
			t.Fatalf("cost column not right-aligned: %v", ends)
		}
	}
	_ = costEnds
}

func TestSessionTree_EstimateIsMarkedAndNeverSummed(t *testing.T) {
	use := Event{
		CorrelationID: "u1", Timestamp: at(time.Minute), SessionID: sessA, Harness: "claude-code",
		Outcome: OutcomeUsage, LinkedDecision: "d1", ToModel: "claude-sonnet-5-5",
		InputTokens: i64(1_000_000), OutputTokens: i64(0),
	}
	ev := []Event{td("d1", sessA, 0, "claude-sonnet-5-5"), use}
	price := func(string, string) (core.Model, bool) { return core.Model{InputM: 3, OutputM: 15}, true }
	out := SessionTree(ev, sessA, TreeOptions{Price: price})
	if !strings.Contains(out, "~$3.00") || !strings.Contains(out, "cost not observed") {
		t.Fatalf("estimate must be marked ~ and kept out of the header total:\n%s", out)
	}
	if out = SessionTree(ev, sessA, TreeOptions{}); strings.Contains(out, "~") {
		t.Fatalf("no price, no estimate:\n%s", out)
	}
}

func TestSessionTree_PausedDecisionsAreRows(t *testing.T) {
	d := td("d1", sessA, 0, "claude-opus-5-5", ReasonClassifierConfident, ReasonPaused)
	d.Outcome = OutcomeBaseline
	out := SessionTree([]Event{d}, sessA, TreeOptions{Paused: true})
	if !strings.Contains(out, "1 subagent") || !strings.Contains(out, "paused") || !strings.Contains(out, "PAUSED") {
		t.Fatalf("paused control-group decision must be a row, header flagged:\n%s", out)
	}
}

func TestSessionTree_NeverCarriesTaskText(t *testing.T) {
	d := td("d1", sessA, 0, "claude-haiku-5-5")
	d.Agent = "refactor the billing module for ACME"
	out := SessionTree([]Event{d}, sessA, TreeOptions{})
	if strings.Contains(out, "ACME") || strings.Contains(out, "billing") {
		t.Fatalf("label must be subagent N, never stored text:\n%s", out)
	}
	b, _ := json.Marshal(BuildSessionTree([]Event{d}, sessA, TreeOptions{}))
	if strings.Contains(string(b), "ACME") {
		t.Fatalf("json leaked text: %s", b)
	}
}

func TestLatestSessionHash(t *testing.T) {
	ev := []Event{td("d1", sessA, 0, "x"), td("d2", sessB, time.Minute, "x"), tuse("d1", sessA, 2*time.Minute, "x", 1, 1)}
	if got := LatestSessionHash(ev); got != sessB {
		t.Fatalf("latest decision session = %q", got)
	}
	if LatestSessionHash(nil) != "" {
		t.Fatal("empty log has no session")
	}
}

func TestPauseMarker_ExpiryAndFailOpen(t *testing.T) {
	t.Setenv("DOWNSHIFT_STATE_DIR", t.TempDir())
	if SessionPaused(sessA) || SessionPaused("") || SessionPaused("../etc/passwd") {
		t.Fatal("no marker means not paused, and a bad id never builds a path")
	}
	if err := PauseSession(sessA); err != nil {
		t.Fatal(err)
	}
	if !SessionPaused(sessA) || SessionPaused(sessB) {
		t.Fatal("pause is per session")
	}
	p, _ := pausePath(sessA)
	old := time.Now().Add(-PauseTTL - time.Minute)
	if err := os.Chtimes(p, old, old); err != nil {
		t.Fatal(err)
	}
	if SessionPaused(sessA) {
		t.Fatal("an expired marker must not pause")
	}
	if err := PauseSession(sessA); err != nil || !SessionPaused(sessA) {
		t.Fatal("pausing again refreshes the TTL")
	}
	if err := ResumeSession(sessA); err != nil || SessionPaused(sessA) {
		t.Fatal("resume removes the marker")
	}
	if err := ResumeSession(sessA); err != nil {
		t.Fatalf("resume without a marker is not an error: %v", err)
	}
	if err := PauseSession("nope"); err == nil {
		t.Fatal("an invalid id must be refused")
	}
	if fi, err := os.Stat(filepath.Dir(p)); err != nil || fi.Mode().Perm()&0o077 != 0 {
		t.Fatalf("marker dir must be private: %v %v", fi, err)
	}
}
