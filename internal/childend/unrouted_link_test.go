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

// A spawn left unrouted (a pause, or a baseline line an older binary wrote) is
// a decision like any routed one: its launch, end and usage must link to it.
// The linker once treated the "baseline" outcome as a cost record and skipped
// it, so such a spawn never had an end record. The report keeps it out of the
// routed figures by its group.
func TestRecord_UnroutedSpawnLinksLikeRoutedSpawn(t *testing.T) {
	cases := []struct {
		name    string
		outcome string
		reasons []string
		group   string
	}{
		{"old control-group baseline line", telemetry.OutcomeBaseline, []string{"CONTROL"}, telemetry.GroupUnrouted},
		{"routed allow", telemetry.OutcomeAllow, nil, telemetry.GroupRouted},
		{"routed rewrite", telemetry.OutcomeRewriteEmitted, nil, telemetry.GroupRouted},
		{"paused baseline", telemetry.OutcomeBaseline, []string{telemetry.ReasonPaused}, telemetry.GroupUnrouted},
	}
	r := catalog.Load()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("DOWNSHIFT_EVENT_LOG", filepath.Join(t.TempDir(), "events.jsonl"))
			const harness, session, call, agent = "claude-code", "sess-1", "toolu_1", "agent-1"
			decision := telemetry.Event{
				CorrelationID: "d1", Timestamp: time.Now().UTC().Add(-time.Minute).Format(time.RFC3339Nano),
				Harness: harness, Outcome: tc.outcome, ReasonCodes: tc.reasons, Verdict: "OK",
				FromModel: "claude-sonnet-5-5", ToModel: "claude-sonnet-5-5",
				SessionID: telemetry.HashSessionID(session), ToolUseHash: telemetry.HashSessionID(call),
			}
			if tc.outcome == telemetry.OutcomeRewriteEmitted {
				decision.Verdict, decision.ToModel = "DOWNSHIFT", "claude-haiku-5-5"
			}
			telemetry.Record(decision)

			// PostToolUse: the harness reports the model it chose and how the call ended.
			childend.Record(hookport.ChildEnd{
				Harness: harness, AgentKey: agent, ToolUseID: call, SessionID: session,
				ResolvedModel: decision.ToModel, Status: telemetry.ChildCompleted,
				Source: hookport.SourcePayload, IsChild: true,
			}, "test", r)
			// SubagentStop: usage read from the child's transcript, joined by agent id.
			res := childend.Record(hookport.ChildEnd{
				Harness: harness, AgentKey: agent, SessionID: session, Model: decision.ToModel,
				Usage:  &hookport.TokenUsage{InputTokens: 1000, OutputTokens: 200},
				Source: hookport.SourceTranscript, IsChild: true,
			}, "test", r)
			if !res.Linked {
				t.Fatalf("usage not linked to the %s decision: %q", tc.name, res.Note)
			}

			events, err := telemetry.ReadEvents()
			if err != nil {
				t.Fatal(err)
			}
			linked := map[string]int{}
			for _, ev := range events {
				if ev.LinkedDecision == "d1" {
					linked[ev.Outcome]++
				}
			}
			for _, want := range []string{telemetry.OutcomeResolved, telemetry.OutcomeChildEnded, telemetry.OutcomeUsage} {
				if linked[want] != 1 {
					t.Errorf("%s records linked to the decision = %d, want 1 (all: %v)", want, linked[want], linked)
				}
			}

			tasks := telemetry.BuildTasks(events)
			if len(tasks) != 1 {
				t.Fatalf("tasks = %d, want 1", len(tasks))
			}
			got := tasks[0]
			if got.Group != tc.group || !got.Completed() || !got.Observed {
				t.Fatalf("task = %+v, want group %s, completed and priced", got, tc.group)
			}
			rep := telemetry.BuildReport(events, 1)
			switch tc.group {
			case telemetry.GroupUnrouted:
				if rep.Excluded != 1 || rep.Routed.Spawns != 0 {
					t.Fatalf("report = excluded %d, routed %+v; want the unrouted spawn excluded", rep.Excluded, rep.Routed)
				}
			case telemetry.GroupRouted:
				if rep.Routed.Completed != 1 {
					t.Fatalf("report routed = %+v, want 1 completed task", rep.Routed)
				}
			}
		})
	}
}
