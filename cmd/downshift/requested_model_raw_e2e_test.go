// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/adapters/cursor"
	"github.com/tiagovilasboas/downshift/internal/core"
)

// The shared runner keeps an unrecognised requested model as a bounded raw
// value so the log can tell "no model sent" from "auto" or an unlisted id.
// Recognised ids stay in requested_model only; empty stays absent.
func TestHookE2E_RequestedModelRawThroughSharedRunner(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	eventLog := filepath.Join(t.TempDir(), "events.jsonl")
	t.Setenv("DOWNSHIFT_EVENT_LOG", eventLog)
	smallID := cmdCat.ModelFor("cursor", core.TierSmall).ID
	midID := cmdCat.ModelFor("cursor", core.TierMid).ID
	frontierID := cmdCat.ModelFor("cursor", core.TierFrontier).ID
	setSessionAllowlist(t, "cursor", []string{smallID, midID, frontierID})

	cases := []struct {
		name          string
		model         string // tool_input.model; "" omits the field
		wantRequested string
		wantRaw       string // "" means the field must be absent
	}{
		{name: "recognised id", model: frontierID, wantRequested: frontierID},
		{name: "unrecognised auto", model: "auto", wantRequested: "unknown", wantRaw: "auto"},
		{name: "unlisted id", model: "vendor/some-new-model-9", wantRequested: "unknown", wantRaw: "vendor/some-new-model-9"},
		{name: "garbage with spaces", model: "please refactor the billing module", wantRequested: "unknown", wantRaw: "invalid"},
		{name: "too long", model: strings.Repeat("a", 65), wantRequested: "unknown", wantRaw: "invalid"},
		{name: "empty", model: "", wantRequested: "unknown"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := os.WriteFile(eventLog, nil, 0o600); err != nil {
				t.Fatal(err)
			}
			toolInput := map[string]any{"task": "rename the userId variable"}
			if tc.model != "" {
				toolInput["model"] = tc.model
			}
			stdin, _ := json.Marshal(map[string]any{
				"hook_event_name": "preToolUse",
				"tool_name":       "Task",
				"tool_input":      toolInput,
			})
			captureStdout(func() {
				runHookAdapter(
					"cursor",
					bytes.NewReader(stdin),
					func(b []byte) (cursor.Event, error) { var e cursor.Event; return e, json.Unmarshal(b, &e) },
					func(e cursor.Event) (any, string, core.Decision) { return cursor.Handle(e, cmdCat) },
					printCursorAllow,
				)
			})
			raw, err := os.ReadFile(eventLog)
			if err != nil {
				t.Fatal(err)
			}
			if bytes.Contains(raw, []byte("refactor the billing")) {
				t.Fatalf("event log contains payload text: %s", raw)
			}
			var ev map[string]any
			if err := json.Unmarshal(bytes.TrimSpace(raw), &ev); err != nil {
				t.Fatalf("one decision event expected, got %q: %v", raw, err)
			}
			if ev["requested_model"] != tc.wantRequested {
				t.Errorf("requested_model = %v, want %q", ev["requested_model"], tc.wantRequested)
			}
			got, present := ev["requested_model_raw"]
			switch {
			case tc.wantRaw == "" && present:
				t.Errorf("requested_model_raw = %v, want absent", got)
			case tc.wantRaw != "" && got != tc.wantRaw:
				t.Errorf("requested_model_raw = %v, want %q", got, tc.wantRaw)
			}
		})
	}
}
