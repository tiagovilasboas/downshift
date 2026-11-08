// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package mainwork

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tiagovilasboas/downshift-ai/internal/core"
	"github.com/tiagovilasboas/downshift-ai/internal/telemetry"
)

type fakeCatalog struct{}

func (fakeCatalog) Lookup(_, id string) (string, int, bool) {
	if id == "big" {
		return id, telemetry.TierRankFrontier, true
	}
	return "", 0, false
}
func (fakeCatalog) IsExactID(string, string) bool  { return false }
func (fakeCatalog) GatewayTier(string) (int, bool) { return 0, false }

// Every harness: recorded only when it declares a main-session tool event,
// and a subagent call is never recorded.
func TestBuild_EveryHarness(t *testing.T) {
	now := time.Date(2026, 11, 3, 0, 0, 0, 0, time.UTC)
	for _, h := range core.AllHarnessCaps() {
		t.Run(h.Harness, func(t *testing.T) {
			if h.Caps.MainWorkObserved() != (len(h.Caps.MainToolCategories) > 0) {
				t.Fatalf("event and categories must be declared together")
			}
			rec, ok := Build(Call{Harness: h.Harness, Tool: "Bash", Session: "s1"}, h.Caps, now)
			if ok != h.Caps.MainWorkObserved() {
				t.Fatalf("recorded=%v observed=%v", ok, h.Caps.MainWorkObserved())
			}
			if ok && (rec.Session != "s1" || rec.Category != "shell" || rec.Harness != h.Harness) {
				t.Errorf("record = %+v", rec)
			}
			if _, ok := Build(Call{Harness: h.Harness, Tool: "Bash", Subagent: true}, h.Caps, now); ok {
				t.Error("a subagent call must not be recorded")
			}
		})
	}
}

func TestBuild_Categories(t *testing.T) {
	caps := core.ClaudeCodeCaps
	cases := map[string]string{"Read": "read", "Grep": "search", "Glob": "search", "Bash": "shell", "Edit": "edit", "WebFetch": "other"}
	for tool, want := range cases {
		rec, ok := Build(Call{Harness: "claude-code", Tool: tool}, caps, time.Now())
		if !ok || rec.Category != want {
			t.Errorf("%s: %+v ok=%v, want category %s", tool, rec, ok, want)
		}
	}
}

// The log holds codes only: the record has no field that could carry text.
func TestAppendReadSummarize_NoText(t *testing.T) {
	p := filepath.Join(t.TempDir(), FileName)
	now := time.Now()
	for _, tool := range []string{"Read", "Bash", "Edit"} {
		rec, _ := Build(Call{Harness: "claude-code", Tool: tool, Session: "s1"}, core.ClaudeCodeCaps, now)
		if err := Append(p, rec); err != nil {
			t.Fatal(err)
		}
	}
	raw, _ := os.ReadFile(p)
	if strings.Contains(string(raw), "/") && strings.Contains(string(raw), "command") {
		t.Errorf("log holds text: %s", raw)
	}
	recs, err := Read(p)
	if err != nil || len(recs) != 3 {
		t.Fatalf("read: %v %d", err, len(recs))
	}
	frontier := func([]Record) map[string]bool { return map[string]bool{"claude-code/s1": true} }
	for _, r := range summarize(recs, 0, now, frontier) {
		switch {
		case r.Harness == "claude-code" && (!r.Observed || r.Mechanical != 2 || r.Frontier != 2):
			t.Errorf("claude-code row = %+v", r)
		case r.Harness != "claude-code" && (r.Observed || r.Mechanical != 0):
			t.Errorf("%s must be unobserved: %+v", r.Harness, r)
		}
	}
}

// The tier comes from the session's latest decision parent model, at report time.
func TestFrontierSessions_LatestParentModelWins(t *testing.T) {
	ev := func(ts, parent string) telemetry.Event {
		return telemetry.Event{Harness: "claude-code", SessionID: "s1", Timestamp: ts, ParentModel: parent}
	}
	got := frontierSessions([]telemetry.Event{ev("2026-11-03T10:00:00Z", "big"), ev("2026-11-03T11:00:00Z", "small")}, fakeCatalog{})
	if got["claude-code/s1"] {
		t.Error("the latest parent model is unknown to the catalog, so the session is not frontier")
	}
	got = frontierSessions([]telemetry.Event{ev("2026-11-03T10:00:00Z", "small"), ev("2026-11-03T11:00:00Z", "big")}, fakeCatalog{})
	if !got["claude-code/s1"] {
		t.Error("latest parent model is frontier")
	}
}
