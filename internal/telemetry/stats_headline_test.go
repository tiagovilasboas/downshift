// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package telemetry_test

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

func headlineEvent(from, to, tier string, actual, baseline *float64) telemetry.Event {
	return telemetry.Event{
		CorrelationID: "h-" + to, Timestamp: time.Now().UTC().Format(time.RFC3339Nano), Harness: "claude-code", Outcome: telemetry.OutcomeRewriteEmitted,
		Complexity: "TRIVIAL", FromModel: from, ToModel: to, Tier: tier, Verdict: "DOWNSHIFT",
		EstimatedSavings: 0.5,
	}
}

// usageFor is the usage record of a headline decision, with tokens.
func usageFor(d telemetry.Event, in, out, cached int64) telemetry.Event {
	return telemetry.Event{Timestamp: d.Timestamp, Harness: "claude-code", Outcome: telemetry.OutcomeUsage, LinkedDecision: d.CorrelationID,
		FromModel: d.FromModel, ToModel: d.ToModel, InputTokens: &in, OutputTokens: &out, CachedTokens: &cached}
}

func TestPrintStats_HeadlineIsTokensSaved(t *testing.T) {
	down := headlineEvent("claude-opus-5-5", "claude-haiku-5-5", "SMALL", nil, nil)
	up := headlineEvent("claude-haiku-5-5", "claude-opus-5-5", "FRONTIER", nil, nil)
	evs := []telemetry.Event{down, usageFor(down, 600, 400, 9000), up, usageFor(up, 100, 100, 0)}
	var b bytes.Buffer
	telemetry.PrintStats(evs, telemetry.StatsOptions{Days: 7}, &b)
	lines := strings.Split(b.String(), "\n")
	if want := "Last 7 days · 2 subagents · saved 83% of work · 100% of cache · 200 on a pricier model"; lines[0] != want {
		t.Errorf("line 1 = %q, want %q", lines[0], want)
	}
	if !strings.HasPrefix(lines[1], "Guardrails held 0 risky moves · frontier rewrites on doubt: ") {
		t.Errorf("line 2 = %q", lines[1])
	}
	if !strings.Contains(lines[2], "haiku") || !strings.Contains(lines[2], "opus") {
		t.Errorf("line 3 = %q, want the model mix", lines[2])
	}
	for _, hidden := range []string{"Normalised", "Honor", "Quota", "By complexity", "$"} {
		if strings.Contains(b.String(), hidden) {
			t.Errorf("%q does not belong in the short stats view:\n%s", hidden, b.String())
		}
	}
}

func TestPrintStats_HeadlineWithoutTokens(t *testing.T) {
	var b bytes.Buffer
	telemetry.PrintStats([]telemetry.Event{headlineEvent("claude-opus-5-5", "claude-haiku-5-5", "SMALL", nil, nil)}, telemetry.StatsOptions{Days: 1}, &b)
	first := strings.SplitN(b.String(), "\n", 2)[0]
	if want := "Last 1 day · 1 subagents · tokens not observed yet: wire SubagentStop with downshift setup"; first != want {
		t.Errorf("line 1 = %q, want %q", first, want)
	}
}

func TestPrintStats_HeadlineByAgentType(t *testing.T) {
	gp := headlineEvent("claude-opus-5-5", "claude-haiku-5-5", "SMALL", nil, nil)
	gp.AgentType = "general-purpose"
	ex := headlineEvent("claude-opus-5-5", "claude-opus-5-5", "FRONTIER", nil, nil)
	ex.CorrelationID, ex.AgentType, ex.Outcome = "h-explore", "Explore", telemetry.OutcomeAllow
	evs := []telemetry.Event{gp, usageFor(gp, 600, 400, 9000), ex, usageFor(ex, 500, 500, 0)}
	var b bytes.Buffer
	telemetry.PrintStats(evs, telemetry.StatsOptions{Days: 7}, &b)
	if !strings.Contains(b.String(), "By type: general-purpose: 100% work · 100% cache") {
		t.Errorf("missing the per agent type line:\n%s", b.String())
	}
}

func TestPrintStats_DetailKeepsEverythingAfterHeadline(t *testing.T) {
	d := headlineEvent("claude-opus-5-5", "claude-haiku-5-5", "SMALL", nil, nil)
	var b bytes.Buffer
	telemetry.PrintStats([]telemetry.Event{d, usageFor(d, 600, 400, 9000)}, telemetry.StatsOptions{Days: 7, Detail: true}, &b)
	for _, want := range []string{"Last 7 days · 1 subagents", "Subagent decisions", "By complexity", "Honor", "Quota", "Tokens rerouted, by harness"} {
		if !strings.Contains(b.String(), want) {
			t.Errorf("--detail must keep %q:\n%s", want, b.String())
		}
	}
	if strings.Contains(b.String(), "$") {
		t.Errorf("--detail prints no dollars (downshift report does):\n%s", b.String())
	}
}
