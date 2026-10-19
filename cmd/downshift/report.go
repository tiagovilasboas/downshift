// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/tiagovilasboas/downshift/internal/catalog"
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
	// Dollars are a valuation of the measured tokens, done now with the
	// current catalog, so a price correction reaches the whole history.
	cat := catalog.Load()
	events = telemetry.RepriceUsage(events, cat.LookupByID)
	rep := telemetry.BuildReport(events, days)
	rep.PricesAsOf = cat.PricesAsOf()
	if asJSON {
		out, err := telemetry.ReportJSON(rep)
		if err != nil {
			fmt.Fprintf(stderr, "error encoding report: %v\n", err)
			return 1
		}
		fmt.Fprintln(stdout, out)
		return 0
	}
	telemetry.PrintReport(stdout, rep)
	return 0
}
