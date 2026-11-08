// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"fmt"
	"io"
	"strings"

	"github.com/tiagovilasboas/downshift-ai/internal/accounting"
	"github.com/tiagovilasboas/downshift-ai/internal/console/reportview"
	"github.com/tiagovilasboas/downshift-ai/internal/telemetry"
)

// styledReportData maps the report to what the styled screen draws. The detail
// tabs reuse the --text printers, so their figures are the --text figures.
func styledReportData(d reportOut, events []telemetry.Event, days int, scope accounting.Scope) reportview.Data {
	rd := buildReportData(d, scope)
	rows := make([]reportview.Row, 0, len(rd.Harnesses))
	for _, h := range rd.Harnesses {
		rows = append(rows, reportview.Row{
			Harness: h.Harness, TokensSaved: h.TokensSaved,
			RoutingObserved: h.RoutingObserved, RoutedSharePct: h.RoutedSharePct, Billable: h.Billable,
			RewrittenSpawns: h.RewrittenSpawns, RewrittenBillable: h.RewrittenBillable,
			PremiumObserved: h.PremiumObserved, NonFrontier: h.NonFrontier, TierKnown: h.TierKnown, PremiumPct: h.PremiumPct,
			Reworked: h.Reworked, Ended: h.Ended, ReworkPct: h.ReworkPct,
		})
	}
	return reportview.Data{
		Header: reportview.Header{
			Label: "Downshift report", Days: rd.Header.Days,
			Commands: rd.Header.Commands, Compacted: rd.Header.Compacted,
		},
		Harnesses: rows,
		Tabs: []reportview.Tab{
			tabOf("Savings", func(w io.Writer) {
				printCompactionLead(w, d.TokensByHarness, scope)
				fmt.Fprintf(w, "\n📦 COMPACTOR (%s)\n", d.Numbers.Scope.Label())
				fmt.Fprintf(w, "   %s\n", d.Numbers.Display.Compactor)
				printCompactionDetailText(w, d.CompactionDetail)
			}),
			tabOf("Routing", func(w io.Writer) {
				telemetry.PrintSegmentedReport(w, events, days)
				printRoutingWhy(w, d.WhySpawnsChanged)
				printMainWork(w, d.MainSessionWork, scope.Label())
			}),
			tabOf("Efficiency", func(w io.Writer) { printEfficiencyText(w, d.Efficiency) }),
			tabOf("Quota", func(w io.Writer) { printQuotaByHarness(w, d.TokensByHarness, scope) }),
		},
	}
}

// tabOf renders one section through its --text printers and splits it into lines.
func tabOf(title string, render func(io.Writer)) reportview.Tab {
	var buf bytes.Buffer
	render(&buf)
	text := strings.Trim(buf.String(), "\n")
	if text == "" {
		return reportview.Tab{Title: title, Lines: []string{}}
	}
	return reportview.Tab{Title: title, Lines: strings.Split(text, "\n")}
}
