// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package childend_test

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/tiagovilasboas/downshift/internal/catalog"
	"github.com/tiagovilasboas/downshift/internal/childend"
	"github.com/tiagovilasboas/downshift/internal/hookport"
	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

// Claude Code PostToolUse carries tool_response.resolvedModel and usage but no
// top-level "model". When resolvedModel refutes the rewrite (harness ran opus,
// not haiku), the usage is still priced on the rewrite target (haiku) and the
// opus-vs-haiku gap is written as a real saving.
func TestHonesty_RefutedRewritePricedOnRewriteTarget(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.jsonl")
	t.Setenv("DOWNSHIFT_EVENT_LOG", path)
	r := catalog.Load()
	now := time.Now().UTC()
	dec := telemetry.Event{CorrelationID: "d1", Timestamp: now.Add(-time.Minute).Format(time.RFC3339Nano),
		Harness: "claude-code", SessionID: telemetry.HashSessionID("sess-1"), ToolUseHash: telemetry.HashSessionID("call-1"),
		Outcome: telemetry.OutcomeRewriteEmitted, Verdict: "DOWNSHIFT",
		FromModel: "claude-opus-5-5", ToModel: "claude-haiku-5-5"}
	if err := telemetry.AppendTo(path, dec); err != nil {
		t.Fatal(err)
	}
	usage := hookport.TokenUsage{InputTokens: 10000, OutputTokens: 2000}
	res := childend.Record(hookport.ChildEnd{Harness: "claude-code", SessionID: "sess-1", ToolUseID: "call-1",
		ResolvedModel: "claude-opus-5-5", Status: telemetry.ChildCompleted, Usage: &usage,
		Source: hookport.SourcePayload, IsChild: true}, "test", r)
	if res.Usage == nil || res.Usage.ActualCostUSD == nil {
		t.Fatalf("precondition: no priced usage written: %+v", res)
	}
	evs, _ := telemetry.ReadEventsFrom(path)
	refuted := false
	for _, ev := range evs {
		if ev.Outcome == telemetry.OutcomeResolved && ev.RewriteHonored != nil && !*ev.RewriteHonored {
			refuted = true
		}
	}
	if !refuted {
		t.Fatal("precondition: no refuting resolved record written")
	}
	opus, _ := r.LookupByID("claude-code", "claude-opus-5-5")
	want := telemetry.CostUSD(opus, usage)
	if got := *res.Usage.ActualCostUSD; got != want {
		t.Fatalf("usage priced on %s at $%.4f (baseline $%.4f, real saved $%.4f); harness resolved opus, want actual $%.4f",
			res.Usage.ToModel, got, *res.Usage.BaselineCostUSD, res.Usage.RealSavedUSD(), want)
	}
}
