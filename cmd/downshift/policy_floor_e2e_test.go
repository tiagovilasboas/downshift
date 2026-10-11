// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/adapters/cursor"
	"github.com/tiagovilasboas/downshift/internal/core"
)

// TestHookE2E_ProjectFloorThroughSharedRunner proves the floor arrives through
// the shared runner (payload cwd -> project context -> core), not through any
// adapter-specific code: a Cursor payload with a cwd holding a policy file is
// routed to the floor tier instead of the small tier.
func TestHookE2E_ProjectFloorThroughSharedRunner(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("DOWNSHIFT_EVENT_LOG", filepath.Join(t.TempDir(), "events.jsonl"))
	smallID := cmdCat.ModelFor("cursor", core.TierSmall).ID
	midID := cmdCat.ModelFor("cursor", core.TierMid).ID
	frontierID := cmdCat.ModelFor("cursor", core.TierFrontier).ID
	setSessionAllowlist(t, "cursor", []string{smallID, midID, frontierID})

	proj := t.TempDir()
	p := filepath.Join(proj, ".downshift", "policy.json")
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(`{"version":1,"min_tier":"mid"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	run := func(cwd string) string {
		payload := map[string]any{
			"hook_event_name": "preToolUse",
			"tool_name":       "Task",
			"model_id":        frontierID,
			"tool_input":      map[string]any{"task": "rename the userId variable to userIdentifier", "model": frontierID},
		}
		if cwd != "" {
			payload["cwd"] = cwd
		}
		stdin, _ := json.Marshal(payload)
		out := captureStdout(func() {
			runHookAdapter(
				bytes.NewReader(stdin),
				func(b []byte) (cursor.Event, error) { var e cursor.Event; return e, json.Unmarshal(b, &e) },
				func(e cursor.Event) (any, string, core.Decision) { return cursor.Handle(e, cmdCat) },
				printCursorAllow,
			)
		})
		var o struct {
			UpdatedInput map[string]any `json:"updated_input"`
		}
		if err := json.Unmarshal([]byte(out), &o); err != nil || o.UpdatedInput == nil {
			t.Fatalf("no rewrite in %q (%v)", out, err)
		}
		model, _ := o.UpdatedInput["model"].(string)
		return model
	}

	if got := run(proj); got != midID {
		t.Errorf("with policy cwd: model = %s, want mid %s", got, midID)
	}
	if got := run(t.TempDir()); got != smallID {
		t.Errorf("without policy: model = %s, want small %s", got, smallID)
	}
}
