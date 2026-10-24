// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package accounting

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/tiagovilasboas/downshift/internal/execwrap"
	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

func ago(d time.Duration) string { return time.Now().UTC().Add(-d).Format(time.RFC3339Nano) }

func i64(n int64) *int64 { return &n }

// cheaperChild is a decision that ran on a cheaper model, with its usage record.
func cheaperChild(session, id string, when time.Duration, work, cache int64) []telemetry.Event {
	return []telemetry.Event{
		{CorrelationID: id, Timestamp: ago(when), SessionID: session, Harness: "claude-code",
			Outcome: telemetry.OutcomeRewriteEmitted, Verdict: "DOWNSHIFT", FromModel: "claude-opus-5-5", ToModel: "claude-haiku-4-5"},
		{Timestamp: ago(when), SessionID: session, Harness: "claude-code", Outcome: telemetry.OutcomeUsage, LinkedDecision: id,
			InputTokens: i64(work), CachedTokens: i64(cache)},
	}
}

func exec(session string, when time.Duration, raw, emitted int64, compacted bool) execwrap.Event {
	return execwrap.Event{TS: time.Now().UTC().Add(-when).Format(time.RFC3339), Session: session,
		RawBytes: raw, EmittedBytes: emitted, Compacted: compacted}
}

func TestCompute_DaysWindowDropsOldExecEvents(t *testing.T) {
	execs := []execwrap.Event{
		exec("", 40*24*time.Hour, 4000, 0, true),            // outside 30 days
		exec("", 1*time.Hour, 400, 100, true),               // inside
		{TS: "not-a-time", RawBytes: 9000, EmittedBytes: 0}, // no parseable time: outside every window
	}
	got := Compute(nil, execs, Scope{Kind: ScopeDays, Days: 30}).Compactor
	if got.Commands != 1 || got.Compacted != 1 || got.SavedBytes != 300 || got.Tokens != 75 {
		t.Errorf("30-day compactor = %+v, want 1 command, 300 saved bytes, 75 tokens", got)
	}
	all := Compute(nil, execs, Scope{Kind: ScopeAll}).Compactor
	if all.Commands != 3 || all.SavedBytes != 4000+300+9000 {
		t.Errorf("all-time compactor = %+v", all)
	}
}

func TestCompute_RoutingAndCompactorShareTheWindow(t *testing.T) {
	var events []telemetry.Event
	events = append(events, cheaperChild("s1", "old", 40*24*time.Hour, 500, 0)...)
	events = append(events, cheaperChild("s1", "new", time.Hour, 100, 1000)...)
	execs := []execwrap.Event{exec("s1", 40*24*time.Hour, 4000, 0, true), exec("s1", time.Hour, 400, 100, true)}

	win := Compute(events, execs, Scope{Kind: ScopeDays, Days: 30})
	if win.Routing.WorkSaved != 100 || win.Routing.CacheSaved != 1000 {
		t.Errorf("windowed routing = %+v, want only the recent child", win.Routing)
	}
	if win.Compactor.Tokens != 75 {
		t.Errorf("windowed compactor tokens = %d, want 75 (same window)", win.Compactor.Tokens)
	}
	if win.TokensSavedEstimate != 100+1000+75 {
		t.Errorf("estimate = %d, want routing work+cache saved plus compactor tokens", win.TokensSavedEstimate)
	}
}

func TestCompute_SessionScope(t *testing.T) {
	var events []telemetry.Event
	events = append(events, cheaperChild("a", "a1", time.Hour, 100, 0)...)
	events = append(events, cheaperChild("b", "b1", time.Hour, 700, 0)...)
	execs := []execwrap.Event{
		exec("a", time.Hour, 400, 0, true),
		exec("b", time.Hour, 800, 0, true),
		exec("", time.Hour, 1200, 0, true), // no session: belongs to no session
	}

	sess := Compute(events, execs, Scope{Kind: ScopeSession, SessionHash: "a"})
	if sess.Routing.WorkSaved != 100 {
		t.Errorf("session routing saved = %d, want 100", sess.Routing.WorkSaved)
	}
	if sess.Compactor.Commands != 1 || sess.Compactor.SavedBytes != 400 {
		t.Errorf("session compactor = %+v, want only session a", sess.Compactor)
	}
	if sess.TokensSavedEstimate != 100+100 {
		t.Errorf("session estimate = %d, want 200", sess.TokensSavedEstimate)
	}

	all := Compute(events, execs, Scope{Kind: ScopeAll})
	if all.Compactor.Commands != 3 || all.Routing.WorkSaved != 800 {
		t.Errorf("all scope = routing %d compactor commands %d, want 800 and 3", all.Routing.WorkSaved, all.Compactor.Commands)
	}
	if none := Compute(events, execs, Scope{Kind: ScopeSession}); none.Routing.WorkSaved != 0 || none.Compactor.Commands != 0 {
		t.Errorf("empty session hash must match nothing, got %+v", none)
	}
}

func TestCompute_SpawnCountsUseThePanelHelpers(t *testing.T) {
	events := []telemetry.Event{
		// model switch
		{CorrelationID: "m", SessionID: "s", Harness: "claude-code", Outcome: telemetry.OutcomeRewriteEmitted,
			Verdict: "DOWNSHIFT", FromModel: "claude-opus-5-5", ToModel: "claude-haiku-4-5"},
		// effort only
		{CorrelationID: "e", SessionID: "s", Harness: "claude-code", Outcome: telemetry.OutcomeRewriteEmitted,
			Verdict: "DOWNSHIFT", FromModel: "claude-opus-5-5", ToModel: "claude-opus-5-5"},
		// held by a guardrail
		{CorrelationID: "g", SessionID: "s", Harness: "claude-code", Outcome: telemetry.OutcomeAllow,
			Verdict: "DOWNSHIFT", FromModel: "claude-opus-5-5", ToModel: "claude-haiku-4-5", Corrections: []string{"R1"}},
	}
	n := Compute(events, nil, Scope{Kind: ScopeAll})
	if n.ModelSwitched != 1 || n.EffortChanged != 1 || n.Held != 1 || n.HeldGuardrail != 1 {
		t.Errorf("counts = switched model %d effort %d held %d guardrail %d, want 1 each",
			n.ModelSwitched, n.EffortChanged, n.Held, n.HeldGuardrail)
	}
}

func TestCompute_DisplayNamesTheWindow(t *testing.T) {
	n := Compute(nil, nil, Scope{Kind: ScopeDays, Days: 30})
	if !strings.HasPrefix(n.Display.TokensSaved, "Tokens saved (estimate): ") || !strings.Contains(n.Display.TokensSaved, "last 30 days") {
		t.Errorf("display = %q, want the estimate and its window", n.Display.TokensSaved)
	}
	if got := Compute(nil, nil, Scope{Kind: ScopeAll}).Display.TokensSaved; !strings.Contains(got, "all time") {
		t.Errorf("all-scope display = %q", got)
	}
}

func TestDisplay_ShortAndScopeLabel(t *testing.T) {
	cases := []struct {
		name  string
		total int64
		want  string
	}{
		{"zero", 0, "0"},
		{"hundreds", 999, "999"},
		{"thousands", 15121, "15k"},
		{"millions", 20_700_000, "20.7M"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			d := displayOf(Numbers{Scope: Scope{Kind: ScopeAll}, TokensSavedEstimate: c.total})
			if d.TokensSavedShort != c.want {
				t.Errorf("tokens_saved_short = %q, want %q", d.TokensSavedShort, c.want)
			}
		})
	}

	scopes := []struct {
		scope Scope
		want  string
	}{
		{Scope{Kind: ScopeDays, Days: 30}, "last 30 days"},
		{Scope{Kind: ScopeSession, SessionHash: "abc"}, "this session"},
		{Scope{Kind: ScopeAll}, "all time"},
	}
	for _, s := range scopes {
		t.Run(s.want, func(t *testing.T) {
			n := Compute(nil, nil, s.scope)
			if n.Display.ScopeLabel != s.want {
				t.Errorf("scope_label = %q, want %q", n.Display.ScopeLabel, s.want)
			}
		})
	}
}

func TestNumbers_JSONKeys(t *testing.T) {
	b, err := json.Marshal(Compute(nil, nil, Scope{Kind: ScopeSession, SessionHash: "abc"}))
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"scope", "routing", "compactor", "model_switched", "effort_changed", "held",
		"held_guardrail", "upshift_not_applied", "tokens_saved_estimate", "display"} {
		if _, ok := m[key]; !ok {
			t.Errorf("Numbers JSON lacks %q: %s", key, b)
		}
	}
	var d map[string]json.RawMessage
	json.Unmarshal(m["display"], &d)
	for _, key := range []string{"tokens_saved", "tokens_saved_short", "scope_label", "routing", "compactor"} {
		if _, ok := d[key]; !ok {
			t.Errorf("display JSON lacks %q", key)
		}
	}
	var c map[string]json.RawMessage
	json.Unmarshal(m["compactor"], &c)
	for _, key := range []string{"commands", "compacted", "raw_bytes", "saved_bytes", "tokens"} {
		if _, ok := c[key]; !ok {
			t.Errorf("compactor JSON lacks %q", key)
		}
	}
}
