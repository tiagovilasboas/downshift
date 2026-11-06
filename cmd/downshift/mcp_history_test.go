// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/core"
	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

// The history adapter reads the session id from the variable each harness
// declares, counts only that session, and reports an unknown session (no
// variable, or a harness without one) instead of falling back to another.
func TestMcpHistory_EveryHarness(t *testing.T) {
	for _, n := range core.AllHarnessCaps() {
		t.Run(n.Harness, func(t *testing.T) {
			env := map[string]string{}
			if n.Caps.SessionEnv != "" {
				env[n.Caps.SessionEnv] = "sess-1"
			}
			mine := telemetry.HashSessionID("sess-1")
			other := telemetry.HashSessionID("sess-2")
			events := []telemetry.Event{
				{Harness: n.Harness, SessionID: mine, Outcome: "allow", Tier: "MID", AgentType: "custom", ReasonCodes: []string{telemetry.ReasonAboveAgentDefault}},
				{Harness: n.Harness, SessionID: mine, Outcome: "allow", Tier: "SMALL"},
				{Harness: n.Harness, SessionID: other, Outcome: "allow", Tier: "FRONTIER"},
			}
			h := mcpHistory{getenv: func(k string) string { return env[k] }, read: func() []telemetry.Event { return events }}
			got, err := h.Summary(context.Background(), n.Harness)
			if err != nil {
				t.Fatal(err)
			}
			if n.Caps.SessionEnv == "" {
				if got.Known || got.Note == "" || got.Spawns != 0 {
					t.Fatalf("no session env: %+v, want unknown with a note", got)
				}
				return
			}
			if !got.Known || got.Spawns != 2 || got.ByTier["mid"] != 1 || got.ByTier["small"] != 1 || got.ByTier["frontier"] != 0 ||
				got.ByAgentType["custom"] != 1 || got.ByAgentType["unspecified"] != 1 || got.AboveAgentDefault != 1 {
				t.Fatalf("summary = %+v", got)
			}
			env[n.Caps.SessionEnv] = ""
			if got, _ := h.Summary(context.Background(), n.Harness); got.Known || got.Spawns != 0 {
				t.Fatalf("unset env must not guess a session: %+v", got)
			}
		})
	}
}

// A session whose events sit before more than 2 MiB of other traffic is still
// counted whole: the reader does not use the statusline tail bound.
func TestReadSessionEvents_NotTailBounded(t *testing.T) {
	log := filepath.Join(t.TempDir(), "events.jsonl")
	t.Setenv("DOWNSHIFT_EVENT_LOG", log)
	mine := telemetry.HashSessionID("sess-1")
	other := telemetry.HashSessionID("sess-2")
	var b strings.Builder
	b.WriteString(`{"harness":"claude-code","session_id":"` + mine + `","outcome":"allow","tier":"MID"}` + "\n")
	filler := `{"harness":"claude-code","session_id":"` + other + `","outcome":"allow","tier":"SMALL"}` + "\n"
	for b.Len() < int(telemetry.DefaultTailBytes)+1024 {
		b.WriteString(filler)
	}
	if err := os.WriteFile(log, []byte(b.String()), 0o600); err != nil {
		t.Fatal(err)
	}
	got := telemetry.SummarizeSpawns(readSessionEvents(), mine)
	if got.Spawns != 1 || got.ByTier["mid"] != 1 {
		t.Fatalf("summary = %+v, want the one early spawn counted", got)
	}
}
