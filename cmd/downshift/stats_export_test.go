// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

// stats --export producing pasteable JSON for multi-user evidence.

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/tiagovilasboas/downshift-ai/internal/adapters/cursor"
	"github.com/tiagovilasboas/downshift-ai/internal/core"
	"github.com/tiagovilasboas/downshift-ai/internal/testenv"
)

func TestStatsExport_PasteableJSON(t *testing.T) {
	testenv.SetHome(t, t.TempDir())
	eventLog := filepath.Join(t.TempDir(), "events.jsonl")
	t.Setenv("DOWNSHIFT_EVENT_LOG", eventLog)

	smallID := cmdCat.ModelFor("cursor", core.TierSmall).ID
	midID := cmdCat.ModelFor("cursor", core.TierMid).ID
	frontierID := cmdCat.ModelFor("cursor", core.TierFrontier).ID
	setSessionAllowlist(t, "cursor", []string{smallID, midID, frontierID})
	stdin := []byte(`{"hook_event_name":"preToolUse","tool_name":"Task","model_id":"` +
		frontierID + `","tool_input":{"task":"rename the userId variable","model":"` + frontierID + `"}}`)
	captureStdout(func() {
		runHookAdapter(
			"cursor",
			bytes.NewReader(stdin),
			func(b []byte) (cursor.Event, error) { var e cursor.Event; return e, json.Unmarshal(b, &e) },
			func(e cursor.Event) (any, string, core.Decision) { return cursor.Handle(e, cmdCat) },
			printCursorAllow,
		)
	})

	out := captureStdout(func() {
		if rc := runStats([]string{"--days=0", "--export"}); rc != 0 {
			t.Errorf("runStats --export rc = %d, want 0", rc)
		}
	})
	var summary struct {
		Total         int     `json:"total"`
		DownshiftRate float64 `json:"downshift_rate"`
		Note          string  `json:"note"`
	}
	if err := json.Unmarshal([]byte(out), &summary); err != nil {
		t.Fatalf("export is not valid JSON: %v\n%s", err, out)
	}
	if summary.Total != 1 || summary.Note == "" {
		t.Errorf("summary = %+v, want total 1 with honesty note", summary)
	}
}
