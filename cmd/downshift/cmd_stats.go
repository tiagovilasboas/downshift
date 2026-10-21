// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/tiagovilasboas/downshift/internal/codeintel"
	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

// runStats reads the local event log and prints a savings summary.
// Usage: downshift stats [--days=N] [--export]
//
// --days=N           time window in days (default 30; 0 = all time)
// --export           print a pasteable JSON summary for multi-user evidence
// --by-harness       Markdown review table: spawns, tier, effort and linked tokens per harness
func runStats(args []string) int {
	opts := telemetry.StatsOptions{Days: 30}
	export := false
	byHarness := false
	for _, arg := range args {
		switch {
		case arg == "--export":
			export = true
		case arg == "--by-harness":
			byHarness = true
		case arg == "--detail":
			opts.Detail = true
		case arg == "--dci":
			opts.ShowDCI = true
		case strings.HasPrefix(arg, "--days=") && len(arg) > len("--days="):
			n, err := strconv.ParseFloat(strings.TrimPrefix(arg, "--days="), 64)
			if err != nil || n < 0 {
				fmt.Fprintf(os.Stderr, "invalid --days value: %s\n", strings.TrimPrefix(arg, "--days="))
				return 2
			}
			opts.Days = int(n)
		default:
			fmt.Fprintf(os.Stderr, "unknown flag: %s\n", arg)
			return 2
		}
	}

	events, err := telemetry.ReadEvents()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error reading event log: %v\n", err)
		return 1
	}
	if byHarness {
		telemetry.PrintCuration(os.Stdout, telemetry.BuildCuration(telemetry.FilterByDays(events, opts.Days)))
		return 0
	}

	dciEvents, _ := codeintel.ReadTelemetryEvents()
	dciFiltered := codeintel.FilterTelemetryByDays(dciEvents, opts.Days)
	dciSummary := codeintel.AggregateTelemetry(dciFiltered)

	if export {
		sum := telemetry.ExportSummary(events, opts.Days, time.Now().UTC())
		if dciSummary.TotalPrompts > 0 {
			sum.DCISummary = &dciSummary
		}
		out, err := telemetry.ExportJSON(sum)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error exporting summary: %v\n", err)
			return 1
		}
		fmt.Println(out)
		return 0
	}
	if len(events) == 0 && dciSummary.TotalPrompts == 0 {
		fmt.Fprintln(os.Stderr, "no events recorded yet: spawn a subagent in a new harness session, or run `downshift doctor` to check the hooks.")
		return 0
	}
	if opts.ShowDCI {
		if dciSummary.TotalPrompts == 0 {
			fmt.Fprintln(os.Stderr, "no DCI prompt events recorded in this window.")
			return 0
		}
		codeintel.PrintDCIStats(os.Stdout, dciSummary)
		return 0
	}
	if len(events) > 0 {
		telemetry.PrintStats(events, opts, os.Stdout)
		if n := telemetry.RotatedBackupCount(); n > 0 {
			fmt.Fprintf(os.Stdout, "Note: %d rotated log backup(s) next to events.jsonl are not included in these totals.\n", n)
			telemetry.PrintBackupLine(os.Stdout)
		}
	}
	if dciSummary.TotalPrompts > 0 {
		if len(events) > 0 {
			fmt.Println()
		}
		codeintel.PrintDCIStats(os.Stdout, dciSummary)
	}
	return 0
}
