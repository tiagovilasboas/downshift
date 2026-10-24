// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/tiagovilasboas/downshift/internal/accounting"
	"github.com/tiagovilasboas/downshift/internal/codeintel"
	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

// defaultReportDays is the window `downshift report` uses without --days. The
// Agents panel uses the same window, so both read this one constant.
const defaultReportDays = 30

// runReport compares routed spawns with the built-in control group: cost per
// completed task (API lens, priced now), tokens saved (plan lens), rework rate
// and savings with a bootstrap interval. It only reads the event log.
func runReport(args []string, stdout, stderr io.Writer) int {
	days := defaultReportDays
	asJSON := false
	for _, arg := range args {
		switch {
		case arg == "--json":
			asJSON = true
		case strings.HasPrefix(arg, "--days="):
			n, err := strconv.ParseFloat(strings.TrimPrefix(arg, "--days="), 64)
			if err != nil || n < 0 {
				fmt.Fprintf(stderr, "invalid --days value: %s\n", strings.TrimPrefix(arg, "--days="))
				return 2
			}
			days = int(n)
		default:
			fmt.Fprintf(stderr, "unknown flag: %s\nusage: downshift report [--days=N] [--json]\n", arg)
			return 2
		}
	}

	events, err := telemetry.ReadEvents()
	if err != nil {
		fmt.Fprintf(stderr, "error reading event log: %v\n", err)
		return 1
	}

	// The exec log is read as a whole and windowed by accounting.Compute, so the
	// routing and the Compactor share the --days window.
	execEvents, _ := accounting.ReadExecEvents()
	nums := accounting.Compute(events, execEvents, accounting.Scope{Kind: accounting.ScopeDays, Days: days})

	rep := telemetry.BuildReport(events, days)
	rep.TokensSaved = nums.Routing
	rep.SetCompactor(nums.Compactor.SavedBytes, nums.Compactor.Tokens)

	if asJSON {
		out, err := json.MarshalIndent(reportOut{Report: rep, Numbers: nums}, "", "  ")
		if err != nil {
			fmt.Fprintf(stderr, "error encoding report: %v\n", err)
			return 1
		}
		fmt.Fprintln(stdout, string(out))
		return 0
	}

	// The routed-vs-control comparison, the plan lens and the per-harness
	// completion lines are the canonical report; telemetry owns their wording
	// (the tests read those strings), so delegate instead of reprinting them.
	telemetry.PrintReport(stdout, rep)

	// Compactor and code-intelligence savings are produced outside the event
	// log, so they are appended here rather than inside telemetry.PrintReport.
	dciEvents, _ := codeintel.ReadTelemetryEvents()
	dciFiltered := codeintel.FilterTelemetryByDays(dciEvents, days)
	dciSummary := codeintel.AggregateTelemetry(dciFiltered)

	fmt.Fprintf(stdout, "\n📦 COMPACTOR (%s)\n", nums.Scope.Label())
	fmt.Fprintf(stdout, "   %s\n", nums.Display.Compactor)

	fmt.Fprintln(stdout, "\n🔍 CODE INTELLIGENCE (DCI)")
	fmt.Fprintf(stdout, "   Prompts: %d · Injections: %d (%.0f%%)\n",
		dciSummary.TotalPrompts, dciSummary.InjectionsApplied, dciSummary.InjectionPct)
	fmt.Fprintf(stdout, "   Context bytes: %d\n", dciSummary.TotalContextBytes)
	if dciSummary.LatencyP50Ms > 0 {
		fmt.Fprintf(stdout, "   Latency p50/p95: %.0fms / %.0fms\n",
			dciSummary.LatencyP50Ms, dciSummary.LatencyP95Ms)
	}

	// The one final figure, in the same window as the parts above; the parts
	// are in `--json`.
	fmt.Fprintf(stdout, "\n%s\n", nums.Display.TokensSaved)

	return 0
}

// reportOut is the `--json` report: the comparison and its keys as before,
// plus the Numbers every surface shares.
type reportOut struct {
	telemetry.Report
	Numbers accounting.Numbers `json:"numbers"`
}
