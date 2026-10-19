// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package telemetry

import (
	"os"
	"strings"
	"testing"
	"time"
)

func TestSpawnStateOf(t *testing.T) {
	cases := []struct {
		name string
		ev   Event
		want string
	}{
		{"rewrite emitted", Event{Outcome: OutcomeRewriteEmitted}, StateSwitched},
		{"allow with nothing to do", Event{Outcome: OutcomeAllow}, StateObserved},
		{"guardrail correction", Event{Outcome: OutcomeAllow, Corrections: []string{"R3"}}, StateHeld},
		{"held beats an emitted outcome", Event{Outcome: OutcomeRewriteEmitted, Corrections: []string{"R8"}}, StateHeld},
		{"held reason code", Event{Outcome: OutcomeAllow, ReasonCodes: []string{"HELD_UNCONFIDENT_UPSHIFT"}}, StateHeld},
		{"credit held", Event{Outcome: OutcomeAllow, CreditHeld: true}, StateHeld},
		{"paused", Event{Outcome: OutcomeBaseline, ReasonCodes: []string{ReasonPaused}}, StatePaused},
		{"control group", Event{Outcome: OutcomeBaseline, ReasonCodes: []string{ReasonControl}}, StateControl},
		{"blocked", Event{Outcome: OutcomeBlocked}, StateBlocked},
	}
	for _, c := range cases {
		if got := SpawnStateOf(c.ev); got != c.want {
			t.Errorf("%s: got %q want %q", c.name, got, c.want)
		}
	}
}

func TestSpawnRecordLine(t *testing.T) {
	cases := []struct {
		name string
		ev   Event
		want string
	}{
		{"model switch with effort", Event{Outcome: OutcomeRewriteEmitted, FromModel: "claude-opus-5-5", ToModel: "claude-haiku-4-5",
			RequestedEffort: "high", FinalEffort: "low", Complexity: "TRIVIAL"}, "switched opus → haiku, effort high → low · TRIVIAL"},
		{"effort only", Event{Outcome: OutcomeRewriteEmitted, FromModel: "gpt-6-sol", ToModel: "gpt-6-sol",
			RequestedEffort: "unknown", FinalEffort: "low"}, "switched effort on gpt-6-sol"},
		{"held says what it would have done", Event{Outcome: OutcomeAllow, FromModel: "claude-opus-5-5", ToModel: "claude-opus-5-5",
			RecommendedModel: "claude-haiku-4-5", ReasonCodes: []string{"HELD_UNCONFIDENT_DOWNSHIFT"}, Complexity: "MEDIUM"},
			"held on opus (would be haiku) [HELD_UNCONFIDENT_DOWNSHIFT] · MEDIUM"},
		{"observed", Event{Outcome: OutcomeAllow, FromModel: "unknown", ToModel: "unknown"}, "observed, spawn unchanged on ?"},
	}
	for _, c := range cases {
		if got := NewSpawnRecord(c.ev).Line(); got != c.want {
			t.Errorf("%s:\n got %q\nwant %q", c.name, got, c.want)
		}
	}
}

func TestWriteLiveCountsAndResetsPerSession(t *testing.T) {
	t.Setenv("DOWNSHIFT_EVENT_LOG", t.TempDir()+"/events.jsonl")
	rec := func(sess, state string) SpawnRecord {
		return SpawnRecord{State: state, Harness: "claude-code", Session: sess}
	}
	for _, r := range []SpawnRecord{rec("aaaaaaaa", StateSwitched), rec("aaaaaaaa", StateHeld), rec("aaaaaaaa", StateSwitched)} {
		if err := WriteLive(r); err != nil {
			t.Fatal(err)
		}
	}
	l, err := ReadLive()
	if err != nil || l.Counts[StateSwitched] != 2 || l.Counts[StateHeld] != 1 || l.Last.State != StateSwitched {
		t.Fatalf("live: %+v err=%v", l, err)
	}
	if !l.Active(time.Now()) || l.Active(time.Now().Add(ActiveWindow+time.Second)) {
		t.Fatal("active only inside the window")
	}
	if err := WriteLive(rec("bbbbbbbb", StateObserved)); err != nil {
		t.Fatal(err)
	}
	if l, _ = ReadLive(); l.Counts[StateSwitched] != 0 || l.Counts[StateObserved] != 1 {
		t.Fatalf("counts must reset on a new session: %+v", l)
	}
}

func TestLiveRecordIsPromptFree(t *testing.T) {
	t.Setenv("DOWNSHIFT_EVENT_LOG", t.TempDir()+"/events.jsonl")
	if err := WriteLive(NewSpawnRecord(Event{Outcome: OutcomeAllow, Harness: "codex", SessionID: strings.Repeat("ab", 32)})); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(LivePath())
	if strings.Contains(string(data), strings.Repeat("ab", 32)) {
		t.Fatalf("the full session hash must not be written: %s", data)
	}
}

func TestActivityLine(t *testing.T) {
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	ev := func(ago time.Duration, outcome string) Event {
		return Event{Harness: "codex", SessionID: strings.Repeat("a", 64), Outcome: outcome, FromModel: "gpt-6-sol", ToModel: "gpt-6-sol",
			Timestamp: now.Add(-ago).Format(time.RFC3339Nano)}
	}
	h := strings.Repeat("a", 64)
	cases := []struct {
		name   string
		events []Event
		want   string
	}{
		{"nothing yet", nil, "○ downshift on · no subagent routed yet"},
		{"recent", []Event{ev(10*time.Second, OutcomeAllow)}, "● downshift active · last spawn 10s ago: observed, spawn unchanged on gpt-6-sol"},
		{"old", []Event{ev(20*time.Minute, OutcomeAllow)}, "○ downshift idle · last spawn 20m0s ago: observed, spawn unchanged on gpt-6-sol"},
	}
	for _, c := range cases {
		if got := ActivityLine(c.events, h, now); got != c.want {
			t.Errorf("%s:\n got %q\nwant %q", c.name, got, c.want)
		}
	}
}

func TestPanelLabelsAndKey(t *testing.T) {
	s := strings.Repeat("a", 64)
	ev := func(o string, mod func(*Event)) Event {
		e := Event{Harness: "claude-code", SessionID: s, Outcome: o, Tier: "MID", FromModel: "claude-opus-5-5", ToModel: "claude-sonnet-5-5",
			Timestamp: time.Now().UTC().Format(time.RFC3339Nano), ToolUseHash: ToolUseKey("toolu_1")}
		if mod != nil {
			mod(&e)
		}
		return e
	}
	events := []Event{
		ev(OutcomeRewriteEmitted, nil),
		ev(OutcomeAllow, func(e *Event) { e.Corrections = []string{"R3"}; e.ToModel = e.FromModel }),
		ev(OutcomeAllow, func(e *Event) { e.ToModel = e.FromModel }),
		ev(OutcomeBaseline, func(e *Event) { e.ToModel = e.FromModel }),
	}
	v := BuildPanel(events, s)
	want := []string{"opus → sonnet (mid)", "held: original model kept", "kept", "control (unrouted)"}
	for i, w := range want {
		if v.Spawns[i].Label != w {
			t.Errorf("spawn %d label %q want %q", i, v.Spawns[i].Label, w)
		}
	}
	if v.Switched != 1 || v.Held != 1 || v.Observed != 1 || v.Tokens.Observed() || v.Spawns[0].ToolUse != ToolUseKey("toolu_1") {
		t.Fatalf("%+v", v)
	}
	// Pinned so the mod's own hash (same recipe) can be checked against it.
	if got := ToolUseKey("toolu_1"); got != HashSessionID("toolu_1") {
		t.Fatalf("ToolUseKey must equal HashSessionID: %s vs %s", got, HashSessionID("toolu_1"))
	}
}

func TestReadLiveRefusesOversizedFile(t *testing.T) {
	t.Setenv("DOWNSHIFT_EVENT_LOG", t.TempDir()+"/events.jsonl")
	if err := os.WriteFile(LivePath(), []byte(`{"session":"`+strings.Repeat("a", maxLiveBytes)+`"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if l, err := ReadLive(); err == nil || l.Session != "" {
		t.Fatalf("oversized live.json must be refused: %+v %v", l, err)
	}
}
