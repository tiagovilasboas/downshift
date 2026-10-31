// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package telemetry

import (
	"strings"
	"testing"
)

// The report counts a completed task from observed usage: a known catalog
// tier, not a price. These tests pin that the price lens is gone and that the
// observed-usage criterion holds across the unhappy paths (no usage, a
// duplicate, and any price text leaking into report output).

func decisionEvent(id, session string) Event {
	return Event{
		CorrelationID: id, Timestamp: "2026-10-15T10:00:00Z", Harness: "claude-code",
		SessionID: session, Outcome: OutcomeRewriteEmitted, Verdict: "DOWNSHIFT",
		FromModel: "claude-opus-5-5", ToModel: "claude-haiku-5-5", Tier: "SMALL",
	}
}

func usageEvent(linked, session string, in int64) Event {
	v := in
	return Event{
		Timestamp: "2026-10-15T10:01:00Z", Harness: "claude-code", SessionID: session,
		Outcome: OutcomeUsage, LinkedDecision: linked, InputTokens: &v, ToModel: "claude-haiku-5-5",
	}
}

// Happy path: a spawn with observed usage counts as one completed task, with
// no price field anywhere in the chain.
func TestBuildTasks_ObservedUsageCountsWithoutPrice(t *testing.T) {
	dec := decisionEvent("d1", "s1")
	key := DecisionKey(dec)
	events := []Event{dec, usageEvent(key, "s1", 1000)}
	tasks := BuildTasks(events)
	if len(tasks) != 1 {
		t.Fatalf("want one task, got %d", len(tasks))
	}
	if !tasks[0].Observed {
		t.Fatalf("observed usage must mark the task observed: %+v", tasks[0])
	}
	if !tasks[0].Completed() {
		t.Fatalf("a usage record is proof the child finished: %+v", tasks[0])
	}
}

// Unhappy path: a spawn with no usage record is not a completed, observed task.
func TestBuildTasks_NoUsageIsNotObserved(t *testing.T) {
	dec := decisionEvent("d1", "s1")
	key := DecisionKey(dec)
	ended := Event{
		Timestamp: "2026-10-15T10:00:30Z", Harness: "claude-code", SessionID: "s1",
		Outcome: OutcomeChildEnded, ChildStatus: ChildCompleted, LinkedDecision: key,
	}
	tasks := BuildTasks([]Event{dec, ended})
	if len(tasks) != 1 {
		t.Fatalf("want one task, got %d", len(tasks))
	}
	if tasks[0].Observed {
		t.Fatalf("a task with no usage record must stay unobserved: %+v", tasks[0])
	}
}

// Unhappy path: two usage records for the same decision count the task once.
func TestBuildTasks_DuplicateUsageCountsOnce(t *testing.T) {
	dec := decisionEvent("d1", "s1")
	key := DecisionKey(dec)
	events := []Event{dec, usageEvent(key, "s1", 1000), usageEvent(key, "s1", 1000)}
	tasks := BuildTasks(events)
	if len(tasks) != 1 {
		t.Fatalf("a duplicate usage must not add a task: got %d tasks", len(tasks))
	}
	if !tasks[0].Observed {
		t.Fatalf("task must be observed: %+v", tasks[0])
	}
}

// Unhappy path: the report prints no price, cost, dollar or gateway-price line,
// even with a gateway usage record in the window.
func TestPrintReport_NoPriceLineEver(t *testing.T) {
	dec := decisionEvent("d1", "s1")
	key := DecisionKey(dec)
	gatewayUsage := usageEvent(key, "s1", 1000)
	gatewayUsage.ToModel = "openai/gpt-x"
	rep := BuildReport([]Event{dec, gatewayUsage}, 0)
	var b strings.Builder
	PrintReport(&b, rep)
	out := b.String()
	for _, banned := range []string{"$", "price", "Price", "dollar", "cost", "Gateway runs"} {
		if strings.Contains(out, banned) {
			t.Errorf("report must not mention %q:\n%s", banned, out)
		}
	}
}
