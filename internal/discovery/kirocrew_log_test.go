// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package discovery

import (
	"strings"
	"testing"
)

// assistantLine builds an assistant record in KiroCrew's on-disk spacing, with
// padBytes of filler content so the line can exceed the old 64 KiB scan cap.
func assistantLine(model string, padBytes int) string {
	return `{"role": "assistant", "content": "` + strings.Repeat("x", padBytes) +
		`", "meta": {"turn_stats": {"model": "` + model + `", "elapsed_ms": 10}}}`
}

// A 100 KB assistant line between two assistant lines must not stop the scan:
// the last model (sonnet-4.6) has to win, not the one before the long line.
func TestInferKiroCrewModel_LongLineDoesNotHideLastModel(t *testing.T) {
	crew := setupCrew(t)
	writeState(t, crew, "sub1", "sess-long", "dashboard:chat-9-1", "", "auto")
	writeSessionLog(t, crew, "dashboard_chat-9-1",
		`{"role": "user", "content": "hi"}`,
		assistantLine("claude-sonnet-4.5", 0),
		assistantLine("claude-opus-4.5", 100<<10),
		`{"role": "user", "content": "more"}`,
		assistantLine("claude-sonnet-4.6", 0),
	)

	got, err := InferKiroCrewModel("sess-long")
	if err != nil || got != "claude-sonnet-4.6" {
		t.Fatalf("got %q, %v; want claude-sonnet-4.6", got, err)
	}
}

// When the last assistant line itself is longer than 64 KiB, its model is the
// current one and must still be returned.
func TestModelFromSessionLog_LongLastLineWins(t *testing.T) {
	crew := setupCrew(t)
	writeSessionLog(t, crew, "dashboard_chat-9-2",
		assistantLine("claude-sonnet-4.5", 0),
		assistantLine("claude-opus-4.5", 200<<10),
	)

	if got := modelFromSessionLog(crew, "dashboard:chat-9-2"); got != "claude-opus-4.5" {
		t.Fatalf("got %q; want claude-opus-4.5", got)
	}
}
