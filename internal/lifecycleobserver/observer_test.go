// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package lifecycleobserver_test

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/tiagovilasboas/downshift/internal/lifecycleobserver"
)

func fixture(event string) lifecycleobserver.Event {
	return lifecycleobserver.Event{
		HookEventName: event,
		SessionID:     "fixture-session",
		TurnID:        "fixture-turn",
		ToolName:      "spawn_agent",
		AgentID:       "fixture-agent",
		AgentType:     "worker",
	}
}

func enabledObserver() lifecycleobserver.Observer {
	return lifecycleobserver.New(lifecycleobserver.Config{
		Enabled:    true,
		HMACKey:    []byte("synthetic-fixture-key-material"),
		ObserverID: "fixture-observer",
		Now: func() time.Time {
			return time.Date(2026, time.September, 30, 12, 0, 0, 0, time.FixedZone("fixture", -3*60*60))
		},
	})
}

func TestObserverIsDisabledByDefault(t *testing.T) {
	got := lifecycleobserver.New(lifecycleobserver.Config{}).Observe([]lifecycleobserver.Event{fixture("PreToolUse")})
	if got != nil {
		t.Fatalf("disabled observer returned %#v", got)
	}
}

func TestObservationUsesOnlySafeVersionedSchema(t *testing.T) {
	got := enabledObserver().Observe([]lifecycleobserver.Event{fixture("PreToolUse"), fixture("SubagentStart"), fixture("SubagentStop")})
	if len(got) != 1 {
		t.Fatalf("observations = %#v, want one", got)
	}
	observation := got[0]
	if observation.SchemaVersion != "harness-downshift.observer.v1" || observation.EvidenceState != "observed" || observation.Lifecycle != "completed" || observation.AssociationState != "unavailable" {
		t.Fatalf("unexpected safe observation %#v", observation)
	}
	if observation.ObservedAt != "2026-09-30T15:00:00Z" || len(observation.ObserverIDHash) != 64 || len(observation.SubjectIDHash) != 64 {
		t.Fatalf("invalid timestamp or HMAC fields %#v", observation)
	}
	encoded, err := json.Marshal(observation)
	if err != nil {
		t.Fatal(err)
	}
	for _, raw := range []string{"fixture-session", "fixture-turn", "fixture-agent", "worker", "fixture-observer"} {
		if strings.Contains(string(encoded), raw) {
			t.Fatalf("raw fixture identifier leaked in output: %s", raw)
		}
	}
	var fields map[string]any
	if err := json.Unmarshal(encoded, &fields); err != nil {
		t.Fatal(err)
	}
	if len(fields) != 7 {
		t.Fatalf("schema leaked optional correlation or extra fields: %#v", fields)
	}
	if _, present := fields["correlation_id"]; present {
		t.Fatalf("correlation must be omitted without a documented deterministic source: %#v", fields)
	}
}

func TestAmbiguousLifecycleOmitsCorrelation(t *testing.T) {
	got := enabledObserver().Observe([]lifecycleobserver.Event{
		fixture("PreToolUse"), fixture("PreToolUse"), fixture("SubagentStart"), fixture("SubagentStop"),
	})
	if len(got) != 1 || got[0].AssociationState != "ambiguous" || got[0].CorrelationID != "" {
		t.Fatalf("observations = %#v, want ambiguous without correlation", got)
	}
}

func TestUnmatchedLifecycleIsUnavailable(t *testing.T) {
	got := enabledObserver().Observe([]lifecycleobserver.Event{fixture("PreToolUse")})
	if len(got) != 1 || got[0].AssociationState != "unavailable" || got[0].Lifecycle != "needs_attention" {
		t.Fatalf("observations = %#v, want unavailable lifecycle", got)
	}
}
