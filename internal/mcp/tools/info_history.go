// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package tools

import "context"

// SessionSummary is the orchestrator's own spawn pattern in this session:
// counts by tier and by agent type, and how many spawns ran above their
// agent's default tier. It holds no task text. Known is false when the
// harness's session id cannot be known to this server; the counts are then
// empty and Note says so, because a guessed session would show another one.
type SessionSummary struct {
	Known             bool           `json:"known"`
	Note              string         `json:"note,omitempty"`
	Spawns            int            `json:"spawns"`
	ByTier            map[string]int `json:"by_tier"`
	ByAgentType       map[string]int `json:"by_agent_type"`
	AboveAgentDefault int            `json:"above_agent_default"`
}

// SessionHistory summarizes the current session's spawns for a harness.
type SessionHistory interface {
	Summary(ctx context.Context, harness string) (SessionSummary, error)
}

// sessionSummary reads the history port; a failure only drops the block.
func (d Deps) sessionSummary(ctx context.Context, harness string) *SessionSummary {
	if d.History == nil {
		return nil
	}
	s, err := d.History.Summary(ctx, harness)
	if err != nil {
		return nil
	}
	return &s
}
