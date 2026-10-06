// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package telemetry_test

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/adapters/claudecode"
	"github.com/tiagovilasboas/downshift/internal/core"
	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

// launchPayload has the shape Claude Code sends to PostToolUse when it
// launches an async subagent (captured from a real session, text removed): no
// token usage, and tool_response.resolvedModel carrying the model it chose.
func launchPayload(session, resolved string) []byte {
	b, _ := json.Marshal(map[string]any{
		"hook_event_name": "PostToolUse",
		"tool_name":       "Agent",
		"session_id":      session,
		"tool_input":      map[string]any{"model": "haiku", "prompt": "x"},
		"tool_response": map[string]any{
			"isAsync":       true,
			"status":        "async_launched",
			"resolvedModel": resolved,
		},
		"duration_ms": 6,
	})
	return b
}

func resolvedSetup(t *testing.T) (log string, small, frontier string, cat core.Resolver) {
	t.Helper()
	c := usageTestCatalog(t)
	small = c.ModelFor("claude-code", core.TierSmall).ID
	frontier = c.ModelFor("claude-code", core.TierFrontier).ID
	log = filepath.Join(t.TempDir(), "events.jsonl")
	t.Setenv("DOWNSHIFT_EVENT_LOG", log)
	return log, small, frontier, c
}

func TestResolved_DatedIDMatchingTheWrittenModelCountsAsHonored(t *testing.T) {
	log, small, frontier, cat := resolvedSetup(t)
	seedDecision(t, log, telemetry.HashSessionID("s1"), frontier, small)

	// The harness reports a dated id; the hook wrote the undated catalog id.
	claudecode.HandlePostToolUse(launchPayload("s1", small+"-20251001"), "test", cat)

	events, _ := telemetry.ReadEvents()
	shifted, honored := telemetry.CountInferredHonored(events)
	if shifted != 1 || honored != 1 {
		t.Fatalf("shifted=%d honored=%d, want 1 1", shifted, honored)
	}
	stats := telemetry.Aggregate(events)
	if stats.Total != 1 || stats.UsageLinked != 0 {
		t.Fatalf("a resolved record must not move decision or usage counts: total=%d usage=%d", stats.Total, stats.UsageLinked)
	}
}

// The harness picked a different model than the hook wrote: observed, not honored.
func TestResolved_DifferentModelIsRecordedAsNotHonored(t *testing.T) {
	log, small, frontier, cat := resolvedSetup(t)
	seedDecision(t, log, telemetry.HashSessionID("s1"), frontier, small)

	claudecode.HandlePostToolUse(launchPayload("s1", frontier), "test", cat)

	events, _ := telemetry.ReadEvents()
	var got *telemetry.Event
	for i := range events {
		if events[i].Outcome == telemetry.OutcomeResolved {
			got = &events[i]
		}
	}
	if got == nil || got.RewriteHonored == nil || *got.RewriteHonored {
		t.Fatalf("want a resolved record with rewrite_honored=false, got %+v", got)
	}
	if _, honored := telemetry.CountInferredHonored(events); honored != 0 {
		t.Fatalf("honored = %d, want 0", honored)
	}
}

// A model the catalog cannot resolve is not guessed at.
func TestResolved_UnknownModelLeavesHonorUnset(t *testing.T) {
	log, small, frontier, cat := resolvedSetup(t)
	seedDecision(t, log, telemetry.HashSessionID("s1"), frontier, small)

	claudecode.HandlePostToolUse(launchPayload("s1", "mystery-model-9"), "test", cat)

	for _, ev := range readAll(t) {
		if ev.Outcome == telemetry.OutcomeResolved && ev.RewriteHonored != nil {
			t.Fatalf("rewrite_honored must stay unset for an unresolvable model, got %v", *ev.RewriteHonored)
		}
	}
}

func TestResolved_NothingRecordedWithoutSessionOrAShift(t *testing.T) {
	log, small, frontier, cat := resolvedSetup(t)

	// No session id in the payload.
	seedDecision(t, log, telemetry.HashSessionID("s1"), frontier, small)
	claudecode.HandlePostToolUse(launchPayload("", small), "test", cat)

	// A session with no decision at all.
	claudecode.HandlePostToolUse(launchPayload("other", small), "test", cat)

	for _, ev := range readAll(t) {
		if ev.Outcome == telemetry.OutcomeResolved {
			t.Fatalf("unexpected resolved record: %+v", ev)
		}
	}
}

func TestResolved_ADecisionIsResolvedOnce(t *testing.T) {
	log, small, frontier, cat := resolvedSetup(t)
	seedDecision(t, log, telemetry.HashSessionID("s1"), frontier, small)

	claudecode.HandlePostToolUse(launchPayload("s1", small), "test", cat)
	claudecode.HandlePostToolUse(launchPayload("s1", small), "test", cat)

	n := 0
	for _, ev := range readAll(t) {
		if ev.Outcome == telemetry.OutcomeResolved {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("resolved records = %d, want 1", n)
	}
}

func readAll(t *testing.T) []telemetry.Event {
	t.Helper()
	events, err := telemetry.ReadEvents()
	if err != nil {
		t.Fatalf("read events: %v", err)
	}
	return events
}
