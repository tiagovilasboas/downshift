// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package telemetry_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

func TestPermissionModeLabel(t *testing.T) {
	cases := []struct{ in, want string }{
		{"", ""},
		{"plan", "plan"},
		{"auto", "auto"},
		{"default", "default"},
		{"acceptEdits", "acceptEdits"},
		{"bypassPermissions", "bypassPermissions"},
		{"dontAsk", "dontAsk"},
		{"Plan", "other"},
		{"/Users/someone/secret-project", "other"},
		{"a future mode", "other"},
	}
	for _, c := range cases {
		if got := telemetry.PermissionModeLabel(c.in); got != c.want {
			t.Errorf("PermissionModeLabel(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// The field round-trips when set, is absent from the JSON when empty, and an
// old log line without it still parses with the mode empty.
func TestEventPermissionModeRoundTrip(t *testing.T) {
	cases := []struct {
		name     string
		ev       telemetry.Event
		wantJSON bool
	}{
		{"with mode", telemetry.Event{Harness: "claude-code", Outcome: telemetry.OutcomeAllow, PermissionMode: "plan"}, true},
		{"without mode", telemetry.Event{Harness: "claude-code", Outcome: telemetry.OutcomeAllow}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			raw, err := json.Marshal(c.ev)
			if err != nil {
				t.Fatal(err)
			}
			if has := bytes.Contains(raw, []byte(`"permission_mode"`)); has != c.wantJSON {
				t.Fatalf("permission_mode present = %v, want %v: %s", has, c.wantJSON, raw)
			}
			var back telemetry.Event
			if err := json.Unmarshal(raw, &back); err != nil {
				t.Fatal(err)
			}
			if back.PermissionMode != c.ev.PermissionMode {
				t.Fatalf("round trip = %q, want %q", back.PermissionMode, c.ev.PermissionMode)
			}
		})
	}
	old := `{"correlation_id":"c1","harness":"claude-code","outcome":"allow","requested_model":"claude-opus-5-5"}`
	var ev telemetry.Event
	if err := json.Unmarshal([]byte(old), &ev); err != nil {
		t.Fatalf("old log line must still parse: %v", err)
	}
	if ev.PermissionMode != "" {
		t.Fatalf("old line gained a mode: %q", ev.PermissionMode)
	}
}

func TestCountPermissionModesAndReport(t *testing.T) {
	events := []telemetry.Event{
		{Harness: "claude-code", Outcome: telemetry.OutcomeRewriteEmitted, PermissionMode: "plan"},
		{Harness: "claude-code", Outcome: telemetry.OutcomeAllow, PermissionMode: "plan"},
		{Harness: "claude-code", Outcome: telemetry.OutcomeAllow, PermissionMode: "auto"},
		{Harness: "claude-code", Outcome: telemetry.OutcomeAllow},                                      // old log: no mode
		{Harness: "claude-code", Outcome: telemetry.OutcomeUsage, PermissionMode: "bypassPermissions"}, // not a decision
	}
	counts := telemetry.CountPermissionModes(events)
	if counts["plan"] != 2 || counts["auto"] != 1 || len(counts) != 2 {
		t.Fatalf("CountPermissionModes = %v", counts)
	}
	rep := telemetry.BuildReport(events, 0)
	var out bytes.Buffer
	telemetry.PrintReport(&out, rep)
	if !strings.Contains(out.String(), "Decisions by permission mode\n  plan") {
		t.Fatalf("report lacks the mode split:\n%s", out.String())
	}
	none := telemetry.BuildReport([]telemetry.Event{{Harness: "claude-code", Outcome: telemetry.OutcomeAllow}}, 0)
	out.Reset()
	telemetry.PrintReport(&out, none)
	if strings.Contains(out.String(), "permission mode") {
		t.Fatalf("a log without modes must not print the section:\n%s", out.String())
	}
}

func TestWhyShowsModeAndQuota(t *testing.T) {
	const sess = "b1b1b1b1b1b1b1b1b1b1b1b1b1b1b1b1b1b1b1b1b1b1b1b1b1b1b1b1b1b1b1b1"
	cases := []struct {
		name    string
		ev      telemetry.Event
		want    []string
		notWant []string
	}{
		{
			name: "mode and quota recorded",
			ev:   telemetry.Event{Harness: "claude-code", Outcome: telemetry.OutcomeAllow, Complexity: "SIMPLE", SessionID: sess, PermissionMode: "plan", QuotaStatus: "available"},
			want: []string{"mode        plan", "quota       available"},
		},
		{
			name:    "old log has neither",
			ev:      telemetry.Event{Harness: "claude-code", Outcome: telemetry.OutcomeAllow, Complexity: "SIMPLE", SessionID: sess},
			notWant: []string{"mode ", "quota "},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			out := telemetry.Why([]telemetry.Event{c.ev}, sess)
			for _, w := range c.want {
				if !strings.Contains(out, w) {
					t.Errorf("missing %q in:\n%s", w, out)
				}
			}
			for _, w := range c.notWant {
				if strings.Contains(out, w) {
					t.Errorf("unexpected %q in:\n%s", w, out)
				}
			}
		})
	}
}
