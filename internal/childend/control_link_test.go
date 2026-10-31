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

// A spawn the control group left unrouted is a decision like any routed one:
// its launch, end and usage must link to it, or `downshift report` can never
// count a completed control task. Before the fix the linker treated the
// control decision's "baseline" outcome as a cost record and skipped it, so
// 0 of 31 control spawns had an end record in 30 days of real use.
func TestRecord_ControlSpawnLinksLikeRoutedSpawn(t *testing.T) {
	cases := []struct {
		name    string
		outcome string
		reasons []string
		group   string
	}{
		{"control group", telemetry.OutcomeBaseline, []string{telemetry.ReasonControl}, telemetry.GroupControl},
		{"routed allow", telemetry.OutcomeAllow, nil, telemetry.GroupRouted},
		{"routed rewrite", telemetry.OutcomeRewriteEmitted, nil, telemetry.GroupRouted},
		{"no-route baseline", telemetry.OutcomeBaseline, nil, telemetry.GroupOther},
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
			case telemetry.GroupControl:
				if rep.Control.Completed != 1 || rep.Control.Ended != 1 {
					t.Fatalf("report control = %+v, want 1 ended and 1 completed task", rep.Control)
				}
			case telemetry.GroupRouted:
				if rep.Routed.Completed != 1 {
					t.Fatalf("report routed = %+v, want 1 completed task", rep.Routed)
				}
			}
		})
	}
}
