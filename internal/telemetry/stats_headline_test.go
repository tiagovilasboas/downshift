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
		EstimatedSavings: 0.5, ActualCostUSD: actual, BaselineCostUSD: baseline,
	}
}

func f(v float64) *float64 { return &v }

func TestPrintStats_HeadlineWithRealCost(t *testing.T) {
	evs := []telemetry.Event{
		headlineEvent("claude-opus-5-5", "claude-haiku-5-5", "SMALL", f(1), f(5)),    // saved 4
		headlineEvent("claude-haiku-5-5", "claude-opus-5-5", "FRONTIER", f(6), f(2)), // cost 4 more
		headlineEvent("claude-opus-5-5", "claude-haiku-5-5", "SMALL", f(1), f(2)),    // saved 1
	}
	var b bytes.Buffer
	telemetry.PrintStats(evs, telemetry.StatsOptions{Days: 7}, &b)
	lines := strings.Split(b.String(), "\n")
	if want := "Last 7 days · 3 subagents · net saved $1.00 (saved $5.00, cost more $4.00)"; lines[0] != want {
		t.Errorf("line 1 = %q, want %q", lines[0], want)
	}
	if !strings.HasPrefix(lines[1], "Guardrails held 0 risky moves · frontier rewrites on doubt: ") {
		t.Errorf("line 2 = %q", lines[1])
	}
	if want := "Models: haiku-5-5 2 · opus-5-5 1"; !strings.Contains(lines[2], "haiku") || !strings.Contains(lines[2], "opus") {
		t.Errorf("line 3 = %q, want something like %q", lines[2], want)
	}
	for _, hidden := range []string{"Normalised", "Honor", "Quota", "By complexity", "Real saved"} {
		if strings.Contains(b.String(), hidden) {
			t.Errorf("%q belongs under --detail:\n%s", hidden, b.String())
		}
	}
}

func TestPrintStats_HeadlineWithoutRealCost(t *testing.T) {
	var b bytes.Buffer
	telemetry.PrintStats([]telemetry.Event{headlineEvent("claude-opus-5-5", "claude-haiku-5-5", "SMALL", nil, nil)}, telemetry.StatsOptions{Days: 1}, &b)
	first := strings.SplitN(b.String(), "\n", 2)[0]
	if want := "Last 1 day · 1 subagents · no real cost yet: wire SubagentStop with downshift setup"; first != want {
		t.Errorf("line 1 = %q, want %q", first, want)
	}
}

func TestPrintStats_HeadlineNetLoss(t *testing.T) {
	var b bytes.Buffer
	telemetry.PrintStats([]telemetry.Event{headlineEvent("claude-haiku-5-5", "claude-opus-5-5", "FRONTIER", f(6), f(2))}, telemetry.StatsOptions{Days: 7}, &b)
	if !strings.Contains(b.String(), "net cost $4.00 more (saved $0.00, cost more $4.00)") {
		t.Errorf("net loss must read as a cost:\n%s", b.String())
	}
}

func TestPrintStats_DetailKeepsEverythingAfterHeadline(t *testing.T) {
	var b bytes.Buffer
	telemetry.PrintStats([]telemetry.Event{headlineEvent("claude-opus-5-5", "claude-haiku-5-5", "SMALL", f(1), f(5))}, telemetry.StatsOptions{Days: 7, Detail: true}, &b)
	for _, want := range []string{"Last 7 days · 1 subagents", "Subagent decisions", "By complexity", "Normalised baseline", "Honor", "Quota", "Real saved"} {
		if !strings.Contains(b.String(), want) {
			t.Errorf("--detail must keep %q:\n%s", want, b.String())
		}
	}
}
