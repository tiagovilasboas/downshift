// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"testing"
	"time"

	"github.com/tiagovilasboas/downshift/internal/accounting"
	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

// fixtureReportOut builds the pieces runReport reads, from the golden fixture.
func fixtureReportOut(t *testing.T, scope accounting.Scope) reportOut {
	t.Helper()
	reportGoldenFixture(t)
	events, err := telemetry.ReadEvents()
	if err != nil {
		t.Fatal(err)
	}
	execEvents, _ := accounting.ReadExecEvents()
	now := time.Now()
	perHarness := accounting.TokensByHarness(events, execEvents, scope, quotaByHarness(now))
	return reportOut{
		Numbers:             accounting.Compute(events, execEvents, scope),
		TokensByHarness:     perHarness,
		CompactionByHarness: compactionRows(perHarness),
		CompactionDetail:    accounting.BuildCompactionDetail(execEvents, nil, scope, now),
		Efficiency:          accounting.BuildEfficiency(events, scope),
		WhySpawnsChanged:    accounting.BuildRoutingWhy(events, scope),
	}
}

func TestBuildReportData_FromGoldenFixture(t *testing.T) {
	scope := accounting.Scope{Kind: accounting.ScopeDays, Days: 0}
	d := buildReportData(fixtureReportOut(t, scope), scope)

	if d.Header.Label != "all time" || d.Header.Commands != 1 || d.Header.Compacted != 1 {
		t.Errorf("header = %+v", d.Header)
	}
	rows := map[string]harnessSummary{}
	for _, h := range d.Harnesses {
		rows[h.Harness] = h
	}
	cc, ok := rows["claude-code"]
	if !ok {
		t.Fatalf("no claude-code row: %+v", d.Harnesses)
	}
	if cc.TokensSaved != 1000 {
		t.Errorf("tokens saved = %d, want 1000", cc.TokensSaved)
	}
	if !cc.PremiumObserved || cc.NonFrontier != 1 || cc.TierKnown != 1 || cc.PremiumPct != 100 {
		t.Errorf("premium = %+v", cc)
	}
	if cc.Ended != 1 || cc.Reworked != 0 || cc.ReworkPct != 0 {
		t.Errorf("rework = %+v", cc)
	}
	// Codex has spawns but no usable session model: unobserved, not 0.
	if codex, ok := rows["codex"]; !ok || codex.RoutingObserved || codex.TokensSaved != 0 {
		t.Errorf("codex row = %+v (present %v)", codex, ok)
	}
	for i := 1; i < len(d.Harnesses); i++ {
		if d.Harnesses[i-1].Harness >= d.Harnesses[i].Harness {
			t.Errorf("harness rows not sorted: %q before %q", d.Harnesses[i-1].Harness, d.Harnesses[i].Harness)
		}
	}
	if len(d.Quota) != len(d.Routing.ByHarness) || len(d.Savings.ByHarness) != len(d.Routing.ByHarness) {
		t.Errorf("sections disagree: quota %d, savings %d, routing %d", len(d.Quota), len(d.Savings.ByHarness), len(d.Routing.ByHarness))
	}
}
