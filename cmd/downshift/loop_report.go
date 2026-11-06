// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/tiagovilasboas/downshift/internal/loop"
	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

const loopReportUsage = `Usage: downshift loop report [--session=<id>] [--json]`

// runLoopReport prints the acceptance summary of the loop and accept runs.
func runLoopReport(args []string, out, errw io.Writer) int {
	session := ""
	asJSON := false
	for _, a := range args {
		switch {
		case strings.HasPrefix(a, "--session="):
			session = strings.TrimPrefix(a, "--session=")
		case a == "--json":
			asJSON = true
		default:
			fmt.Fprintf(errw, "loop report: unknown argument %q\n%s\n", a, loopReportUsage)
			return 2
		}
	}
	events, err := telemetry.ReadEvents()
	if err != nil {
		fmt.Fprintf(errw, "loop report: %v\n", err)
		return 1
	}
	r := loop.BuildReport(events, session)
	if asJSON {
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		_ = enc.Encode(r)
		return 0
	}
	fmt.Fprintf(out, "Acceptance: %d used, %d discarded, %d unknown\n", r.Used, r.Discarded, r.Unknown)
	if r.AcceptanceRate == nil {
		fmt.Fprintln(out, "Rate:       no run decided yet")
	} else {
		fmt.Fprintf(out, "Rate:       %.0f%% of decided runs were used\n", *r.AcceptanceRate*100)
	}
	for _, s := range r.Sessions {
		fmt.Fprintf(out, "  %s: %d used, %d discarded, %d unknown\n", s.SessionID, s.Used, s.Discarded, s.Unknown)
	}
	return 0
}
