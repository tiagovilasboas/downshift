// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package opencode_test

// Observation-side tests (wire v1): arrival-order resolution, the negative
// control (no switch chain, no claim), the refuted case, dropped-line
// accounting, ParseChildEnd, and the usage seam that stays unobserved
// (NOT PROVEN — see the package comment in port.go). Fixtures are synthetic:
// fixed hex correlation ids, catalog model ids, placeholder statuses.

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/adapters/opencode"
	"github.com/tiagovilasboas/downshift/internal/catalog"
	"github.com/tiagovilasboas/downshift/internal/core"
	"github.com/tiagovilasboas/downshift/internal/core/decidertest"
	"github.com/tiagovilasboas/downshift/internal/hookport"
	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

const otherChild = "ffffffffffffffffffffffffffffffff"

// portEnv isolates the observation side: fresh HOME and state dir, and a
// private event log. Returns the log path.
func portEnv(t *testing.T) string {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("DOWNSHIFT_STATE_DIR", t.TempDir())
	log := filepath.Join(t.TempDir(), "events.jsonl")
	t.Setenv("DOWNSHIFT_EVENT_LOG", log)
	return log
}

// obsLine builds one valid observation envelope for testChild.
func obsLine(t *testing.T, event string, fields map[string]any) string {
	t.Helper()
	m := map[string]any{
		"v":           1,
		"kind":        "observation",
		"correlation": map[string]any{"child": testChild},
		"event":       event,
	}
	for k, v := range fields {
		m[k] = v
	}
	b, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func batch(lines ...string) []byte {
	return []byte(strings.Join(lines, "\n") + "\n")
}

// seedRoutedDecision writes the decision event a real hook run would have
// left: a routed opencode decision whose session and tool-use hashes are the
// spawn correlation (the one pair the port links by, see hookrun.go).
func seedRoutedDecision(t *testing.T, small, mid, frontier string, cat *catalog.Catalog) {
	t.Helper()
	// The routed decision is the precondition, not the subject: pin it.
	decidertest.Use(t, decidertest.Tier(core.TierSmall))
	allowlist(t, small, mid, frontier)
	resp, _, d := opencode.Handle(request(trivialRename, frontier), cat)
	if resp.Action != "route" || d.Model.ID != small {
		t.Fatalf("seed decision did not route: resp=%+v decision=%+v", resp, d)
	}
	ev := telemetry.FromDecision(d, "seed-correlation", "test")
	ev.SessionID = telemetry.HashSessionID(testChild)
	ev.ToolUseHash = telemetry.HashSessionID(testChild)
	telemetry.Record(ev)
}

func readEvents(t *testing.T, log string) []telemetry.Event {
	t.Helper()
	events, err := telemetry.ReadEventsFrom(log)
	if err != nil {
		t.Fatal(err)
	}
	return events
}

func resolvedEvent(t *testing.T, events []telemetry.Event) telemetry.Event {
	t.Helper()
	for _, ev := range events {
		if ev.Outcome == telemetry.OutcomeResolved {
			return ev
		}
	}
	t.Fatalf("no resolved record written: %+v", events)
	return telemetry.Event{}
}

func TestHonor_ArrivalOrderResolvesToTheDecisionTarget(t *testing.T) {
	log := portEnv(t)
	small, mid, frontier, cat := tierIDs(t)
	seedRoutedDecision(t, small, mid, frontier, cat)
	raw := batch(
		obsLine(t, "switch_attempt", map[string]any{"from": frontier, "to": small, "result": "ok"}),
		obsLine(t, "model_selected", map[string]any{"model": small, "previous": frontier}),
		obsLine(t, "child_context", map[string]any{"model": small}),
	)
	stdout, note := opencode.Honor(raw, "test", cat)
	if stdout != nil || note != "" {
		t.Fatalf("honor returned stdout=%v note=%q", stdout, note)
	}
	resolved := resolvedEvent(t, readEvents(t, log))
	if resolved.RewriteHonored == nil || !*resolved.RewriteHonored {
		t.Fatalf("expected the resolution confirmed against the decision target, got %+v", resolved)
	}
	if resolved.ToModel != small {
		t.Fatalf("resolved model = %q, want %s", resolved.ToModel, small)
	}
	if resolved.LinkedDecision == "" {
		t.Fatalf("resolved record must link its decision: %+v", resolved)
	}
}

func TestHonor_NoSwitchChainNoClaim(t *testing.T) {
	cases := []struct {
		name  string
		lines []string
	}{
		{"selection without a switch", []string{
			obsLine(t, "model_selected", map[string]any{"model": "big-pickle", "previous": "glm-5.3-flash-free"}),
			obsLine(t, "child_context", map[string]any{"model": "big-pickle"}),
		}},
		{"switch without a selection", []string{
			obsLine(t, "switch_attempt", map[string]any{"result": "ok"}),
			obsLine(t, "child_context", map[string]any{"model": "big-pickle"}),
		}},
		{"context before the switch", []string{
			obsLine(t, "child_context", map[string]any{"model": "big-pickle"}),
			obsLine(t, "switch_attempt", map[string]any{"result": "ok"}),
			obsLine(t, "model_selected", map[string]any{"model": "big-pickle"}),
		}},
		{"switch that failed", []string{
			obsLine(t, "switch_attempt", map[string]any{"result": "error"}),
			obsLine(t, "model_selected", map[string]any{"model": "big-pickle"}),
			obsLine(t, "child_context", map[string]any{"model": "big-pickle"}),
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			log := portEnv(t)
			small, mid, frontier, cat := tierIDs(t)
			seedRoutedDecision(t, small, mid, frontier, cat)
			stdout, note := opencode.Honor(batch(tc.lines...), "test", cat)
			if stdout != nil {
				t.Fatalf("unobserved chain returned stdout=%v", stdout)
			}
			_ = note // dropped-line accounting is not the subject here
			events := readEvents(t, log)
			for _, ev := range events {
				if ev.Outcome == telemetry.OutcomeResolved {
					t.Fatalf("no chain, no claim: resolved record fabricated: %+v", ev)
				}
			}
			if len(events) != 1 {
				t.Fatalf("expected only the seeded decision, got %d events", len(events))
			}
		})
	}
}

func TestHonor_ResolvedMismatchRefutesTheRewrite(t *testing.T) {
	log := portEnv(t)
	small, mid, frontier, cat := tierIDs(t)
	seedRoutedDecision(t, small, mid, frontier, cat)
	raw := batch(
		obsLine(t, "switch_attempt", map[string]any{"from": frontier, "to": small, "result": "ok"}),
		obsLine(t, "model_selected", map[string]any{"model": small, "previous": frontier}),
		obsLine(t, "child_context", map[string]any{"model": mid}), // landed elsewhere
	)
	if _, note := opencode.Honor(raw, "test", cat); note != "" {
		t.Fatalf("unexpected note %q", note)
	}
	resolved := resolvedEvent(t, readEvents(t, log))
	if resolved.RewriteHonored == nil || *resolved.RewriteHonored {
		t.Fatalf("a mismatch must be refuted, got %+v", resolved)
	}
	if resolved.ToModel != mid {
		t.Fatalf("resolved model = %q, want %s", resolved.ToModel, mid)
	}
}

func TestHonor_NegativeControlWritesNothing(t *testing.T) {
	log := portEnv(t)
	small, mid, frontier, cat := tierIDs(t)
	seedRoutedDecision(t, small, mid, frontier, cat)
	raw := batch(
		obsLine(t, "model_selected", map[string]any{"model": small}),
		obsLine(t, "child_context", map[string]any{"model": small}),
		obsLine(t, "child_done", map[string]any{"model": small, "status": "completed"}),
	)
	stdout, note := opencode.Honor(raw, "test", cat)
	if stdout != nil || note != "" {
		t.Fatalf("negative control returned stdout=%v note=%q", stdout, note)
	}
	events := readEvents(t, log)
	if len(events) != 1 || events[0].Outcome == telemetry.OutcomeResolved {
		t.Fatalf("without the switch chain nothing is recorded: %+v", events)
	}
}

func TestHonor_DroppedLinesAreCounted(t *testing.T) {
	log := portEnv(t)
	small, mid, frontier, cat := tierIDs(t)
	seedRoutedDecision(t, small, mid, frontier, cat)
	other := `{"v":1,"kind":"observation","correlation":{"child":"` + otherChild + `"},"event":"child_context","model":"` + mid + `"}`
	raw := batch(
		`{"v":1,"kind":"observation",`,
		obsLine(t, "switch_attempt", map[string]any{"result": "ok"}),
		obsLine(t, "model_selected", map[string]any{"model": small, "previous": frontier}),
		obsLine(t, "child_context", map[string]any{"model": small}),
		other,
	)
	if _, note := opencode.Honor(raw, "test", cat); note != "2 invalid observation line(s) dropped" {
		t.Fatalf("note = %q", note)
	}
	resolved := resolvedEvent(t, readEvents(t, log))
	if resolved.RewriteHonored == nil || !*resolved.RewriteHonored {
		t.Fatalf("dropped lines must not affect the claim: %+v", resolved)
	}
}

func TestParseChildEnd_FirstDoneWinsWithUsageMapping(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	small, mid, _, _ := tierIDs(t)
	done := obsLine(t, "child_done", map[string]any{
		"model":  small,
		"status": "completed",
		"usage": map[string]any{
			"input": 1200, "output": 340, "reasoning": 80,
			"cache": map[string]any{"read": 500, "write": 100},
		},
	})
	second := `{"v":1,"kind":"observation","correlation":{"child":"` + otherChild + `"},"event":"child_done","model":"` + mid + `","status":"failed"}`
	raw := batch(
		obsLine(t, "switch_attempt", map[string]any{"result": "ok"}),
		done,
		second,
	)
	ce, ok := opencode.ParseChildEnd(raw)
	if !ok {
		t.Fatal("ParseChildEnd() rejected a valid child_done batch")
	}
	if ce.Harness != "opencode" || ce.AgentKey != testChild || ce.ToolUseID != testChild || ce.SessionID != testChild {
		t.Fatalf("identity fields = %+v", ce)
	}
	if ce.Model != small {
		t.Fatalf("end model = %q, want %s", ce.Model, small)
	}
	if ce.Status != telemetry.ChildCompleted {
		t.Fatalf("status = %q, want %q", ce.Status, telemetry.ChildCompleted)
	}
	if ce.Usage == nil || ce.Usage.InputTokens != 1200 || ce.Usage.OutputTokens != 340 ||
		ce.Usage.CachedTokens != 600 || ce.Usage.CacheWriteTokens != 100 {
		t.Fatalf("usage mapping = %+v", ce.Usage)
	}
	if ce.ResolvedModel != "" {
		t.Fatalf("the end parser must never claim a resolved model here: %+v", ce)
	}
	if ce.Source != hookport.SourcePayload || !ce.IsChild {
		t.Fatalf("child-end markers = %+v", ce)
	}
}

func TestParseChildEnd_RejectsBadInput(t *testing.T) {
	valid := func(fields map[string]any) string { return obsLine(t, "child_done", fields) }
	cases := []struct {
		name string
		raw  []byte
	}{
		{"empty", nil},
		{"malformed only", []byte("{\"v\":1,\nnot json\n")},
		{"no child_done", batch(obsLine(t, "child_context", map[string]any{"model": "big-pickle"}))},
		{"non-hex child", []byte(`{"v":1,"kind":"observation","correlation":{"child":"zzzz"},"event":"child_done"}`)},
		{"negative usage", batch(valid(map[string]any{"usage": map[string]any{"input": -1}}))},
		{"absurd usage", batch(valid(map[string]any{"usage": map[string]any{"output": int64(1)<<41 + 1}}))},
		{"usage wrong type", batch(valid(map[string]any{"usage": map[string]any{"input": "lots"}}))},
		{"status over cap", batch(valid(map[string]any{"status": strings.Repeat("s", 33)}))},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, ok := opencode.ParseChildEnd(tc.raw); ok {
				t.Fatalf("ParseChildEnd(%q) accepted a payload it must reject", tc.raw)
			}
		})
	}
}

func TestParseChildEnd_DropsUnknownObservationFields(t *testing.T) {
	small, _, _, _ := tierIDs(t)
	line := obsLine(t, "child_done", map[string]any{
		"model":   small,
		"status":  "done",
		"raw":     "canary-7b91d",
		"parent":  "canary-7b91d",
		"payload": map[string]any{"prompt": "canary-7b91d"},
	})
	ce, ok := opencode.ParseChildEnd(batch(line))
	if !ok {
		t.Fatal("unknown observation fields must be dropped, not rejected")
	}
	b, err := json.Marshal(ce)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "canary-7b91d") {
		t.Fatalf("unknown fields leaked into the ChildEnd: %s", b)
	}
	if ce.Model != small {
		t.Fatalf("known field lost: %+v", ce)
	}
}

func TestPort_HonorIsObservedUsageIsNot(t *testing.T) {
	portEnv(t)
	_, _, _, cat := tierIDs(t)
	port := opencode.Port()
	if port.ID != "opencode" || port.Honor == nil || port.ParseChildEnd == nil {
		t.Fatalf("port seam incomplete: %+v", port)
	}
	// The token seam is NOT PROVEN (see port.go): no usage func is
	// registered, so usage reads Observed=false — never a measured zero.
	if port.Usage != nil || port.UsageOut != nil {
		t.Fatalf("usage must stay unregistered: %+v", port)
	}
	obs := hookport.ObserveUsage(port, []byte(`{"v":1}`), "test", cat)
	if obs.Observed || obs.Stdout != nil || obs.Note != "" {
		t.Fatalf("an unregistered usage seam is Observed=false, not a zero: %+v", obs)
	}
	// A nil port func is the same absence, for reference.
	if empty := hookport.ObserveHonor(hookport.Port{}, []byte(`{}`), "test", cat); empty.Observed {
		t.Fatalf("a nil Honor func must read Observed=false: %+v", empty)
	}
	// The real honor func is observed even when the batch yields no claim.
	honor := hookport.ObserveHonor(port, []byte("not json\n"), "test", cat)
	if !honor.Observed {
		t.Fatalf("a batch the port ran is Observed=true: %+v", honor)
	}
}
