// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: BUSL-1.1
// Commercial use requires a licence — see LICENSE for terms.

package main

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/tiagovilasboas/harness-downshift/internal/adapters/cursor"
	"github.com/tiagovilasboas/harness-downshift/internal/core"
	"github.com/tiagovilasboas/harness-downshift/internal/telemetry"
)

// A routing decision whose hook response leaves the spawn unchanged must be
// recorded as "allow", not "rewrite_emitted" (which feeds honored-rewrite
// evidence).
func TestHookTelemetry_NoRewriteRecordsAllow(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	eventLog := filepath.Join(t.TempDir(), "events.jsonl")
	t.Setenv("DOWNSHIFT_EVENT_LOG", eventLog)

	frontierID := cmdCat.ModelFor("cursor", core.TierFrontier).ID
	setSessionAllowlist(t, "cursor", []string{frontierID}) // nothing cheaper to rewrite to

	stdin := []byte(`{"hook_event_name":"preToolUse","tool_name":"Task","model_id":"` +
		frontierID + `","tool_input":{"task":"rename the userId variable","model":"` + frontierID + `"}}`)
	out := captureStdout(func() {
		runHookAdapter(
			bytes.NewReader(stdin),
			func(b []byte) (cursor.Event, error) { var e cursor.Event; return e, json.Unmarshal(b, &e) },
			func(e cursor.Event) (any, string, core.Decision) { return cursor.Handle(e, cmdCat) },
			printCursorAllow,
		)
	})
	var hookOut struct {
		UpdatedInput map[string]any `json:"updated_input"`
	}
	if err := json.Unmarshal([]byte(out), &hookOut); err != nil || hookOut.UpdatedInput != nil {
		t.Fatalf("precondition: expected an unchanged spawn, got %s (err %v)", out, err)
	}
	events, err := telemetry.ReadEventsFrom(eventLog)
	if err != nil || len(events) != 1 {
		t.Fatalf("events = %d, err = %v; want 1", len(events), err)
	}
	if events[0].Outcome != telemetry.OutcomeAllow {
		t.Fatalf("Outcome = %q, want %q", events[0].Outcome, telemetry.OutcomeAllow)
	}
}
