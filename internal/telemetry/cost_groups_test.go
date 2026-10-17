// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
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

func TestCostGroupsPrintedOnlyWithAttributableUsage(t *testing.T) {
	var none bytes.Buffer
	telemetry.PrintStats([]telemetry.Event{usageEv("", "", 0.2)}, telemetry.StatsOptions{Detail: true}, &none)
	if strings.Contains(none.String(), "Sessions with cost") {
		t.Fatalf("no attributable usage must print no group section:\n%s", none.String())
	}
	var out bytes.Buffer
	telemetry.PrintStats([]telemetry.Event{usageEv("sessAAAA1", "agent1", 0.4), usageEv("sessBBBB2", "agent2", 0.1)}, telemetry.StatsOptions{Detail: true}, &out)
	got := out.String()
	for _, want := range []string{"Sessions with cost", "Subagents with cost", "session sessAAAA", "catalog-priced"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
	if strings.Index(got, "sessAAAA") > strings.Index(got, "sessBBBB") {
		t.Error("sessions must be ordered by cost, highest first")
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
