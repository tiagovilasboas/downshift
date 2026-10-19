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

func usageEv(session, agent string, usd float64) telemetry.Event {
	c := usd
	return telemetry.Event{
		Outcome: telemetry.OutcomeUsage, Harness: "claude-code",
		SessionID: session, AgentHash: agent, ActualCostUSD: &c,
	}
}

func TestAggregateGroupsCostBySessionAndSubagent(t *testing.T) {
	s := telemetry.Aggregate([]telemetry.Event{
		usageEv("sessAAAA1", "agent1", 0.10),
		usageEv("sessAAAA1", "agent2", 0.30),
		usageEv("sessBBBB2", "agent3", 0.05),
		usageEv("", "", 0.50), // unattributable: counted in the total only
	})
	if s.ActualCostEvents != 4 {
		t.Fatalf("total cost events = %d, want 4", s.ActualCostEvents)
	}
	if len(s.CostBySession) != 2 || len(s.CostBySubagent) != 3 {
		t.Fatalf("groups: sessions %d subagents %d", len(s.CostBySession), len(s.CostBySubagent))
	}
	if g := s.CostBySession["sessAAAA1"]; g.Events != 2 || g.USD < 0.399 || g.USD > 0.401 {
		t.Fatalf("session A = %+v", g)
	}
}

// Cost groups stay in the aggregate and the export; `stats` itself prints no
// dollar figure, so --detail shows none of them.
func TestCostGroupsAreNotPrintedByStats(t *testing.T) {
	var out bytes.Buffer
	telemetry.PrintStats([]telemetry.Event{usageEv("sessAAAA1", "agent1", 0.4), usageEv("sessBBBB2", "agent2", 0.1)}, telemetry.StatsOptions{Detail: true}, &out)
	if got := out.String(); strings.Contains(got, "Sessions with cost") || strings.Contains(got, "$") {
		t.Errorf("stats --detail prints dollars:\n%s", got)
	}
}

func TestExportCarriesCostGroupAggregatesOnly(t *testing.T) {
	exp := telemetry.ExportSummary([]telemetry.Event{usageEv("s1", "a1", 0.4), usageEv("s2", "a2", 0.1)}, 0, time.Now())
	if exp.CostSessions != 2 || exp.CostSubagents != 2 || exp.MaxSessionCostUSD == nil || exp.MaxSubagentCostUSD == nil ||
		*exp.MaxSessionCostUSD < 0.399 || *exp.MaxSubagentCostUSD < 0.399 {
		t.Fatalf("export = %+v", exp)
	}
}

func TestUsageWithoutIDsKeepsOutputUnchanged(t *testing.T) {
	var a, b bytes.Buffer
	telemetry.PrintStats(nil, telemetry.StatsOptions{Detail: true}, &a)
	telemetry.PrintStats([]telemetry.Event{}, telemetry.StatsOptions{Detail: true}, &b)
	if a.String() != b.String() {
		t.Fatal("empty inputs must print the same")
	}
}
