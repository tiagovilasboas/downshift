// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/tiagovilasboas/downshift/internal/accounting"
	"github.com/tiagovilasboas/downshift/internal/operatorexport"
	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

// runExport is `downshift export [--days=N]`: the one artefact an outside
// operator shares after a measurement week (roadmap P3.2, issue #104). It
// prints counts and token totals as JSON, reusing the same report aggregation
// `downshift report` reads, and refuses to emit anything the allowlist does not
// pass. It is a sibling of `downshift learning export` (the compressor-learning
// store dump), which is a different concern and stays as it is.
func runExport(args []string, stdout, stderr io.Writer) int {
	days := defaultReportDays
	for _, arg := range args {
		switch {
		case strings.HasPrefix(arg, "--days="):
			n, err := strconv.ParseFloat(strings.TrimPrefix(arg, "--days="), 64)
			if err != nil || n < 0 {
				fmt.Fprintf(stderr, "invalid --days value: %s\n", strings.TrimPrefix(arg, "--days="))
				return 2
			}
			days = int(n)
		default:
			fmt.Fprintf(stderr, "unknown flag: %s\nusage: downshift export [--days=N]\n", arg)
			return 2
		}
	}

	events, err := telemetry.ReadEvents()
	if err != nil {
		fmt.Fprintf(stderr, "error reading event log: %v\n", err)
		return 1
	}
	execEvents, _ := accounting.ReadExecEvents()

	e := operatorexport.Build(events, execEvents, days, time.Now().UTC())

	// Runtime sensor: never emit an export the allowlist cannot vouch for. The
	// package test is the CI sensor; this is the one that guards a real run.
	if err := operatorexport.Check(e); err != nil {
		fmt.Fprintf(stderr, "refusing to print export: %v\n", err)
		return 1
	}

	out, err := operatorexport.JSON(e)
	if err != nil {
		fmt.Fprintf(stderr, "error encoding export: %v\n", err)
		return 1
	}
	fmt.Fprintln(stdout, out)
	return 0
}
