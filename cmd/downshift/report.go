// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/tiagovilasboas/downshift/internal/codeintel"
	"github.com/tiagovilasboas/downshift/internal/execwrap"
	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

// runReport compares routed spawns with the built-in control group: cost per
// completed task (API lens, priced now), tokens saved (plan lens), rework rate
// and savings with a bootstrap interval. It only reads the event log.
func runReport(args []string, stdout, stderr io.Writer) int {
	days := 30
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

	rep := telemetry.BuildReport(events, days)
	// Compactor totals come from the exec log, not the event log, and are not
	// windowed. They feed the one final figure, tokens saved (estimate).
	var compactor execwrap.Summary
	if path, err := execwrap.EventsPath(); err == nil {
		compactor, _ = execwrap.Summarize(path)
	}
	rep.SetCompactor(compactor.SavedBytes, compactor.EstimatedTokensSaved)

	if asJSON {
		out, err := telemetry.ReportJSON(rep)
		if err != nil {
			fmt.Fprintf(stderr, "error encoding report: %v\n", err)
			return 1
		}
		fmt.Fprintln(stdout, out)
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

	fmt.Fprintln(stdout, "\n📦 COMPACTOR")
	fmt.Fprintf(stdout, "   Commands: %d · Compacted: %d\n", compactor.Commands, compactor.Compacted)

	fmt.Fprintln(stdout, "\n🔍 CODE INTELLIGENCE (DCI)")
	fmt.Fprintf(stdout, "   Prompts: %d · Injections: %d (%.0f%%)\n",
		dciSummary.TotalPrompts, dciSummary.InjectionsApplied, dciSummary.InjectionPct)
	fmt.Fprintf(stdout, "   Context bytes: %d\n", dciSummary.TotalContextBytes)
	if dciSummary.LatencyP50Ms > 0 {
		fmt.Fprintf(stdout, "   Latency p50/p95: %.0fms / %.0fms\n",
			dciSummary.LatencyP50Ms, dciSummary.LatencyP95Ms)
	}

	// The one final figure; the parts are in `--json`.
	fmt.Fprintf(stdout, "\n%s\n", telemetry.TokensSavedEstimateLine(rep.TokensSavedEstimate))

	return 0
}
