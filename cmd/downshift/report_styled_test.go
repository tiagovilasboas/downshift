// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"io"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/accounting"
	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

func TestStyledReportData_FromGoldenFixture(t *testing.T) {
	scope := accounting.Scope{Kind: accounting.ScopeDays, Days: 0}
	d := fixtureReportOut(t, scope)
	events, err := telemetry.ReadEvents()
	if err != nil {
		t.Fatal(err)
	}
	got := styledReportData(d, events, 0, scope)

	if got.Header.Commands != 1 || got.Header.Compacted != 1 {
		t.Errorf("header = %+v", got.Header)
	}
	rows := map[string]int{}
	for i, r := range got.Harnesses {
		rows[r.Harness] = i
	}
	cc, ok := rows["claude-code"]
	if !ok || got.Harnesses[cc].TokensSaved != 1000 || !got.Harnesses[cc].PremiumObserved {
		t.Errorf("claude-code row = %+v (present %v)", got.Harnesses, ok)
	}
	// An unobserved harness stays a row of its own and is flagged, not zeroed away.
	codex, ok := rows["codex"]
	if !ok || got.Harnesses[codex].RoutingObserved {
		t.Errorf("codex row = %+v (present %v)", got.Harnesses, ok)
	}
	var titles []string
	for _, tab := range got.Tabs {
		titles = append(titles, tab.Title)
		if len(tab.Lines) == 0 {
			t.Errorf("tab %s has no lines", tab.Title)
		}
	}
	if strings.Join(titles, ",") != "Savings,Routing,Efficiency,Quota" {
		t.Errorf("tabs = %v", titles)
	}
	if lines := strings.Join(got.Tabs[3].Lines, "\n"); !strings.Contains(lines, "QUOTA USED PER HARNESS") {
		t.Errorf("quota tab = %q", lines)
	}
}

func TestTabOf_EmptySectionHasNoLines(t *testing.T) {
	tab := tabOf("Empty", func(w io.Writer) {})
	if tab.Title != "Empty" || len(tab.Lines) != 0 {
		t.Errorf("tab = %+v", tab)
	}
}
