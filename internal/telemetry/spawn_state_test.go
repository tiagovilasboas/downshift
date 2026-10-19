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
	rec := func(sess, state string) SpawnRecord { return SpawnRecord{State: state, Harness: "claude-code", Session: sess} }
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
