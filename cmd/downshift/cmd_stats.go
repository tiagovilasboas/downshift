// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

// runStats reads the local event log and prints a savings summary.
// Usage: downshift stats [--days=N] [--cost-per-unit=USD] [--export]
//
// --days=N           time window in days (default 30; 0 = all time)
// --cost-per-unit=X  convert normalised units to dollars at rate X per unit
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
		case strings.HasPrefix(arg, "--days=") && len(arg) > len("--days="):
			n, err := strconv.ParseFloat(strings.TrimPrefix(arg, "--days="), 64)
			if err != nil || n < 0 {
				fmt.Fprintf(os.Stderr, "invalid --days value: %s\n", strings.TrimPrefix(arg, "--days="))
				return 2
			}
			opts.Days = int(n)
		case strings.HasPrefix(arg, "--cost-per-unit=") && len(arg) > len("--cost-per-unit="):
			v, err := strconv.ParseFloat(strings.TrimPrefix(arg, "--cost-per-unit="), 64)
			if err != nil || v < 0 {
				fmt.Fprintf(os.Stderr, "invalid --cost-per-unit value: %s\n", strings.TrimPrefix(arg, "--cost-per-unit="))
				return 2
			}
			opts.CostPerUnit = v
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
	if export {
		out, err := telemetry.ExportJSON(telemetry.ExportSummary(events, opts.Days, time.Now().UTC()))
		if err != nil {
			fmt.Fprintf(os.Stderr, "error exporting summary: %v\n", err)
			return 1
		}
		fmt.Println(out)
		return 0
	}
	if len(events) == 0 {
		fmt.Fprintln(os.Stderr, "no events recorded yet: spawn a subagent in a new harness session, or run `downshift doctor` to check the hooks.")
		return 0
	}
	telemetry.PrintStats(events, opts, os.Stdout)
	return 0
}
