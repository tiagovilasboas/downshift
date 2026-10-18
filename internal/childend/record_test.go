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

func TestRecord_NoHarnessOrNoUsageRecordsNothing(t *testing.T) {
	t.Setenv("DOWNSHIFT_EVENT_LOG", filepath.Join(t.TempDir(), "events.jsonl"))
	res := childend.Record(hookport.ChildEnd{AgentKey: "a1", Usage: &hookport.TokenUsage{InputTokens: 5}}, "test", catalog.Load())
	if res.Usage != nil {
		t.Fatal("a ChildEnd with no harness was recorded")
	}
	res = childend.Record(hookport.ChildEnd{Harness: "codex", AgentKey: "a1", Source: hookport.SourceTranscript}, "test", catalog.Load())
	if res.Usage != nil || res.Note != "" {
		t.Fatalf("a stop with no usage recorded %+v", res)
	}
	// A transcript-sourced stop cannot be deduped or joined without an agent id.
	res = childend.Record(hookport.ChildEnd{Harness: "codex", Source: hookport.SourceTranscript, Usage: &hookport.TokenUsage{InputTokens: 5}}, "test", catalog.Load())
	if res.Usage != nil {
		t.Fatal("a stop with no agent id was priced")
	}
	if all, _ := telemetry.ReadEvents(); len(all) != 0 {
		t.Fatalf("%d events recorded", len(all))
	}
}

func TestLinkTargetBy_IsPerHarness(t *testing.T) {
	now := time.Now().UTC()
	sess := telemetry.HashSessionID("sess-1")
	prior := []telemetry.Event{
		{CorrelationID: "cx", Harness: "codex", Outcome: telemetry.OutcomeAllow, SessionID: sess, Timestamp: now.Add(-time.Minute).Format(time.RFC3339Nano)},
		{CorrelationID: "cc", Harness: "claude-code", Outcome: telemetry.OutcomeAllow, SessionID: sess, Timestamp: now.Add(-time.Second).Format(time.RFC3339Nano)},
	}
	if got := childend.LinkTargetBy("codex", prior, sess, "", "", now, telemetry.OutcomeUsage); got == nil || got.CorrelationID != "cx" {
		t.Fatalf("codex linked to %+v", got)
	}
	if got := childend.LinkTargetBy("cursor", prior, sess, "", "", now, telemetry.OutcomeUsage); got != nil {
		t.Fatalf("cursor linked to %+v", got)
	}
}
