// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"

	"github.com/tiagovilasboas/downshift/internal/core"
	"github.com/tiagovilasboas/downshift/internal/mcp/tools"
	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

// noSessionNote is what providers says when the session cannot be known.
const noSessionNote = "this harness gives the server no session id, so the session's spawns cannot be summarized"

// mcpHistory adapts the decision events of the current session to the tools
// port. The session id comes from the variable the harness declares in
// HarnessCapabilities.SessionEnv, read from the server's own environment; with
// none set the summary says so and counts nothing (it never falls back to the
// latest session, which could be another one).
type mcpHistory struct {
	getenv func(string) string
	read   func() []telemetry.Event
}

// readSessionEvents reads the whole event log: a long session must not be
// undercounted by a tail bound meant for a statusline render. The summary
// filters by session hash.
func readSessionEvents() []telemetry.Event {
	events, _ := telemetry.ReadEvents()
	return events
}

func (h mcpHistory) Summary(_ context.Context, harness string) (tools.SessionSummary, error) {
	caps, _ := core.CapsFor(harness)
	hash := ""
	if caps.SessionEnv != "" {
		hash = telemetry.HashSessionID(h.getenv(caps.SessionEnv))
	}
	if hash == "" {
		return tools.SessionSummary{Note: noSessionNote, ByTier: map[string]int{}, ByAgentType: map[string]int{}}, nil
	}
	s := telemetry.SummarizeSpawns(h.read(), hash)
	return tools.SessionSummary{Known: true, Spawns: s.Spawns, ByTier: s.ByTier, ByAgentType: s.ByAgentType, AboveAgentDefault: s.AboveAgentDefault}, nil
}
