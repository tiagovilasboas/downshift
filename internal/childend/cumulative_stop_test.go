// Copyright (c) 2026 Tiago Vilas Boas.
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

// A child can stop, be sent another message and stop again. Its transcript is
// cumulative, so each stop records only the growth since the last record: the
// second stop used to be dropped as "already priced", which left a long child
// at a fraction of its real cost (one at $4.17 of $18.94). A repeated stop with
// nothing new records nothing.
func TestRecord_ResumedChildRecordsOnlyGrowth(t *testing.T) {
	t.Setenv("DOWNSHIFT_EVENT_LOG", filepath.Join(t.TempDir(), "events.jsonl"))
	r := catalog.Load()
	const harness, session, call, agent = "claude-code", "sess-1", "toolu_1", "agent-1"
	telemetry.Record(telemetry.Event{
		CorrelationID: "d1", Timestamp: time.Now().UTC().Add(-time.Minute).Format(time.RFC3339Nano),
		Harness: harness, Outcome: telemetry.OutcomeRewriteEmitted, Verdict: "DOWNSHIFT",
		FromModel: "claude-opus-5-5", ToModel: "claude-sonnet-5-5",
		SessionID: telemetry.HashSessionID(session), ToolUseHash: telemetry.HashSessionID(call),
	})
	childend.Record(hookport.ChildEnd{
		Harness: harness, AgentKey: agent, ToolUseID: call, SessionID: session,
		ResolvedModel: "claude-sonnet-5-5", Source: hookport.SourcePayload, IsChild: true,
	}, "test", r)

	stop := func(total hookport.TokenUsage) childend.Result {
		return childend.Record(hookport.ChildEnd{
			Harness: harness, AgentKey: agent, SessionID: session, Model: "claude-sonnet-5-5",
			Usage: &total, Cumulative: true, Source: hookport.SourceTranscript, IsChild: true,
		}, "test", r)
	}
	first := stop(hookport.TokenUsage{InputTokens: 1000, OutputTokens: 200, CachedTokens: 5000})
	if first.Usage == nil || first.Line == "" {
		t.Fatalf("the first stop must be priced and announced: %+v", first)
	}
	if again := stop(hookport.TokenUsage{InputTokens: 1000, OutputTokens: 200, CachedTokens: 5000}); again.Usage != nil {
		t.Fatalf("a repeated stop with nothing new was recorded: %+v", again.Usage)
	}
	resumed := stop(hookport.TokenUsage{InputTokens: 1600, OutputTokens: 500, CachedTokens: 9000})
	if resumed.Usage == nil {
		t.Fatal("the resumed child's growth was dropped")
	}
	if resumed.Line != "" {
		t.Errorf("the child-end line is shown once per child, got %q for the second stop", resumed.Line)
	}
	if !resumed.Linked {
		t.Errorf("the growth must link to the same decision: %q", resumed.Note)
	}

	events, _ := telemetry.ReadEvents()
	var in, out, cached int64
	records := 0
	for _, ev := range events {
		if ev.Outcome != telemetry.OutcomeUsage {
			continue
		}
		records++
		in += *ev.InputTokens
		out += *ev.OutputTokens
		cached += *ev.CachedTokens
	}
	if records != 2 || in != 1600 || out != 500 || cached != 9000 {
		t.Fatalf("usage records = %d with %d/%d/%d tokens, want 2 records summing to the final transcript 1600/500/9000", records, in, out, cached)
	}

	// A count that is not cumulative (one turn's usage) keeps the old once-only rule.
	once := childend.Record(hookport.ChildEnd{
		Harness: harness, AgentKey: agent, SessionID: session, Model: "claude-sonnet-5-5",
		Usage: &hookport.TokenUsage{InputTokens: 9999}, Source: hookport.SourceTranscript, IsChild: true,
	}, "test", r)
	if once.Usage != nil {
		t.Errorf("a non-cumulative count for an agent already priced must not be recorded again: %+v", once.Usage)
	}
}
