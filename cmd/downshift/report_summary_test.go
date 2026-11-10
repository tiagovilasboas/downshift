// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"testing"
	"time"

	"github.com/tiagovilasboas/downshift-ai/internal/accounting"
	"github.com/tiagovilasboas/downshift-ai/internal/execwrap"
)

// Each harness gets its own row, from its own runs only: hook runs carry a
// session, direct calls do not, and read-back bytes are already subtracted.
func TestPaneRows_PerHarnessHookDirectAndReadBack(t *testing.T) {
	now := time.Now().UTC().Format(time.RFC3339)
	hook := execwrap.Event{TS: now, Harness: "claude-code", Session: "s1", RawBytes: 4400, EmittedBytes: 400, Compacted: true, RetrievedBytes: 400}
	direct := execwrap.Event{TS: now, Harness: "claude-code", RawBytes: 1000, EmittedBytes: 1000}
	codex := execwrap.Event{TS: now, Harness: "codex", Session: "s2", RawBytes: 800, EmittedBytes: 0, Compacted: true}
	rows := buildPaneRows(nil, []execwrap.Event{hook, direct, codex}, accounting.Scope{Kind: accounting.ScopeDays, Days: 30})
	if len(rows) != 2 || rows[0].Harness != "claude-code" || rows[1].Harness != "codex" {
		t.Fatalf("rows = %+v", rows)
	}
	c := rows[0]
	if c.Commands != 2 || c.Compacted != 1 || c.HookCommands != 1 || c.DirectCommands != 1 {
		t.Errorf("claude-code runs = %+v", c)
	}
	if c.SavedBytes != 3600 || c.TokensSaved != 900 || c.RetrievedBytes != 400 || c.RawBytes != 5400 {
		t.Errorf("claude-code bytes = %+v, want 3600 net saved of 5400 raw", c)
	}
	if rows[1].TokensSaved != 200 || rows[1].CutPct != 100 {
		t.Errorf("codex = %+v", rows[1])
	}
}
