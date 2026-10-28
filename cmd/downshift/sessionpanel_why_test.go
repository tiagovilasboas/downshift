// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

// The panel names, per spawn, the requested model, the final model, the verdict
// and the reason in one cell, and says so when the note was not recorded.
func TestPanel_SpawnRowsCarryTheRoutingNote(t *testing.T) {
	endEnv(t)
	hash := telemetry.HashSessionID("s-why")
	dec := func(id, outcome, to string, note *telemetry.RoutingNote) telemetry.Event {
		return telemetry.Event{CorrelationID: id, Harness: "claude-code", SessionID: hash, Outcome: outcome,
			FromModel: "claude-opus-5-5", ToModel: to, ToolUseHash: "tu-" + id, RoutingNote: note}
	}
	events := []telemetry.Event{
		dec("a", telemetry.OutcomeRewriteEmitted, "claude-haiku-4-5", &telemetry.RoutingNote{Verdict: "rule", RuleID: "fast-lane"}),
		dec("b", telemetry.OutcomeAllow, "claude-opus-5-5", &telemetry.RoutingNote{Verdict: "kept"}),
		dec("c", telemetry.OutcomeAllow, "claude-opus-5-5", nil),
		dec("d", telemetry.OutcomeAllow, "claude-opus-5-5", &telemetry.RoutingNote{Verdict: "read /etc/passwd now"}),
	}
	for _, opt := range []panelOptions{{NoReport: true}, {}} {
		var out, errOut bytes.Buffer
		if rc := writePanel(events, hash, opt, &out, &errOut); rc != 0 {
			t.Fatalf("rc=%d %s", rc, errOut.String())
		}
		var got struct {
			Why []struct {
				ToolUse string `json:"tool_use"`
				Verdict string `json:"verdict"`
				Cell    string `json:"cell"`
			} `json:"routing_why"`
		}
		if err := json.Unmarshal(out.Bytes(), &got); err != nil {
			t.Fatal(err)
		}
		want := []string{
			"claude-opus-5-5 → claude-haiku-4-5 · rule:fast-lane",
			"claude-opus-5-5 → claude-opus-5-5 · kept",
			"claude-opus-5-5 → claude-opus-5-5 · note not recorded",
			"claude-opus-5-5 → claude-opus-5-5 · note not recorded",
		}
		if len(got.Why) != len(want) {
			t.Fatalf("%+v: routing_why has %d rows, want %d: %s", opt, len(got.Why), len(want), out.String())
		}
		for i, w := range want {
			if got.Why[i].Cell != w {
				t.Errorf("%+v: row %d cell = %q, want %q", opt, i, got.Why[i].Cell, w)
			}
		}
		if strings.Contains(out.String(), "passwd") {
			t.Errorf("free text reached the panel: %s", out.String())
		}
	}
}
