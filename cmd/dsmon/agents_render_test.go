// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

func appendTo(t *testing.T, path, s string) {
	t.Helper()
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if _, err := f.WriteString(s); err != nil {
		t.Fatal(err)
	}
}

func agentLine(ts time.Time, tool, task string) string {
	return fmt.Sprintf(`{"timestamp":%q,"tool":%q,"task":%q,"model":"haiku"}`+"\n", ts.UTC().Format(time.RFC3339Nano), tool, task)
}

// agents.jsonl is written by another process: lines arrive over time, the
// file may be rotated under us, and a line with neither a tool nor a task is
// noise.
func TestPollAgents_ReadsAppendsSkipsNoiseAndSurvivesRotation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "agents.jsonl")
	s := &state{}
	s.pollAgents(path) // missing file: nothing happens
	if len(s.agents) != 0 {
		t.Fatalf("agents from a missing file: %v", s.agents)
	}

	now := time.Now()
	appendTo(t, path, agentLine(now, "Task", "")+`{"timestamp":"x"}`+"\n"+"not json\n")
	s.pollAgents(path)
	if len(s.agents) != 1 || s.agents[0].Tool != "Task" {
		t.Fatalf("agents = %+v, want only the line that has a tool", s.agents)
	}

	s.pollAgents(path) // unchanged size: not read twice
	appendTo(t, path, agentLine(now, "Agent", "rename"))
	s.pollAgents(path)
	if len(s.agents) != 2 {
		t.Fatalf("after append: %d agents, want 2", len(s.agents))
	}

	// Rotation: the file shrinks, state restarts from the new content.
	if err := os.WriteFile(path, []byte(agentLine(now, "Fresh", "")), 0o600); err != nil {
		t.Fatal(err)
	}
	s.pollAgents(path)
	if len(s.agents) != 1 || s.agents[0].Tool != "Fresh" {
		t.Fatalf("after rotation: %+v, want only the new file's line", s.agents)
	}
}

func TestPollAgents_BoundsMemory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "agents.jsonl")
	var b strings.Builder
	now := time.Now()
	for i := 0; i < maxKeptAgents+50; i++ {
		b.WriteString(agentLine(now, fmt.Sprintf("t%d", i), ""))
	}
	appendTo(t, path, b.String())
	s := &state{}
	s.pollAgents(path)
	if len(s.agents) != maxKeptAgents {
		t.Fatalf("kept %d agents, want %d", len(s.agents), maxKeptAgents)
	}
	if last := s.agents[len(s.agents)-1].Tool; last != fmt.Sprintf("t%d", maxKeptAgents+49) {
		t.Fatalf("kept the wrong end: last = %s", last)
	}
}

func TestRecentAgents_LastSixWithinTwentyFourHours(t *testing.T) {
	now := time.Now()
	s := &state{}
	s.agents = append(s.agents, agentEvent{Timestamp: now.Add(-48 * time.Hour).UTC().Format(time.RFC3339Nano), Tool: "old"})
	for i := 0; i < 8; i++ {
		s.agents = append(s.agents, agentEvent{Timestamp: now.UTC().Format(time.RFC3339Nano), Tool: fmt.Sprintf("n%d", i)})
	}
	got := s.recentAgents()
	if len(got) != 6 || got[0].Tool != "n2" || got[5].Tool != "n7" {
		t.Fatalf("recent = %+v, want n2..n7 and no old entry", got)
	}
}

func TestAgeAndLocalTimeFormats(t *testing.T) {
	now := time.Now()
	stamp := func(d time.Duration) string { return now.Add(-d).UTC().Format(time.RFC3339Nano) }

	a := func(d time.Duration) string { return agentEvent{Timestamp: stamp(d)}.age() }
	if got := a(10 * time.Second); !strings.HasSuffix(got, "s") {
		t.Errorf("seconds = %q", got)
	}
	if got := a(5 * time.Minute); !strings.HasSuffix(got, "m") {
		t.Errorf("minutes = %q", got)
	}
	if got := a(3 * time.Hour); len(got) != 5 || got[2] != ':' {
		t.Errorf("hours = %q, want HH:MM", got)
	}
	if got := (agentEvent{Timestamp: "garbage"}).age(); got != "?" {
		t.Errorf("bad timestamp = %q", got)
	}

	l := func(ts string) string { return event{Event: telemetry.Event{Timestamp: ts}}.localTime() }
	if got := l(stamp(5 * time.Second)); !strings.HasSuffix(got, "s") {
		t.Errorf("recent event = %q", got)
	}
	if got := l("garbage"); got != "??:??" {
		t.Errorf("bad timestamp = %q", got)
	}
	if got := l("2026"); got != "??:??" {
		t.Errorf("short timestamp = %q", got)
	}
	if (agentEvent{Timestamp: "garbage"}).isRecent() || (event{Event: telemetry.Event{Timestamp: "garbage"}}).isRecent() {
		t.Error("an unparseable timestamp is never recent")
	}
}

func TestStripANSIAndRowKeepTheBoxAligned(t *testing.T) {
	colored := grn + "ok" + rst + " " + dim + "dim" + rst
	if got := stripANSI(colored); got != "ok dim" {
		t.Fatalf("stripANSI = %q", got)
	}
	for _, content := range []string{"", colored, strings.Repeat("x", boxWidth*2)} {
		r := []rune(stripANSI(row(content)))
		if content != strings.Repeat("x", boxWidth*2) && len(r) != boxWidth {
			t.Errorf("row width = %d, want %d for %q", len(r), boxWidth, content)
		}
		if r[0] != '│' || r[len(r)-1] != '│' && len(content) < boxWidth {
			t.Errorf("row is not boxed: %q", string(r))
		}
	}
	if d := []rune(stripANSI(divider("Agents"))); len(d) != boxWidth {
		t.Errorf("divider width = %d, want %d", len(d), boxWidth)
	}
}

func TestShortModelAndSwitchLine(t *testing.T) {
	for in, want := range map[string]string{
		"claude-haiku-4-5": "haiku  ", "claude-sonnet-5-5": "sonnet ", "claude-opus-5-5": "opus   ",
		"gpt-6-luna": "gpt-6-l", "x": "x      ",
	} {
		if got := shortModel(in); got != want {
			t.Errorf("shortModel(%q) = %q, want %q", in, got, want)
		}
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	mk := func(verdict string) string {
		return stripANSI(switchLine(event{Event: telemetry.Event{
			Timestamp: now, Verdict: verdict, FromModel: "claude-opus-5-5", ToModel: "claude-haiku-4-5",
			Complexity: "TRIVIAL-LONG-NAME", EstimatedSavings: 0.75,
		}}))
	}
	if got := mk("DOWNSHIFT"); !strings.Contains(got, "opus") || !strings.Contains(got, "haiku") || !strings.Contains(got, "-75%") {
		t.Errorf("downshift line = %q", got)
	}
	if got := mk("UPSHIFT"); !strings.Contains(got, "↑") {
		t.Errorf("upshift line = %q", got)
	}
	if got := mk("HELD"); !strings.Contains(got, "held") {
		t.Errorf("held line = %q", got)
	}
	if got := mk("OK"); !strings.Contains(got, "✓") {
		t.Errorf("ok line = %q", got)
	}
}

// render prints the whole widget; every boxed line keeps the box width and an
// empty state does not panic.
func TestRender_BoxStaysAlignedWithAndWithoutData(t *testing.T) {
	ts := time.Now().UTC().Format(time.RFC3339Nano)
	full := &state{lastEvent: time.Now().Add(-90 * time.Second), tick: 3}
	full.all = []event{
		ev(telemetry.Event{Timestamp: ts, Harness: "claude-code", Verdict: "DOWNSHIFT", EstimatedSavings: 0.8, Outcome: "rewrite_emitted", FromModel: "opus", ToModel: "haiku"}),
		ev(telemetry.Event{Timestamp: ts, Harness: "codex", Verdict: "OK", Outcome: "allow"}),
	}
	full.agents = []agentEvent{{Timestamp: ts, Tool: "Task", Task: "rename", Model: "haiku"}}
	for name, s := range map[string]*state{"empty": {}, "full": full} {
		out := captureStdout(t, func() { render(s) })
		if !strings.Contains(out, "│") {
			t.Fatalf("%s: no box drawn:\n%s", name, out)
		}
		for _, line := range strings.Split(stripANSI(out), "\n") {
			if strings.HasPrefix(line, "│") && len([]rune(line)) > boxWidth {
				t.Errorf("%s: line wider than the box (%d): %q", name, len([]rune(line)), line)
			}
		}
	}
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	done := make(chan string)
	go func() { b, _ := io.ReadAll(r); done <- string(b) }()
	defer func() { os.Stdout = old }()
	fn()
	w.Close()
	return <-done
}
