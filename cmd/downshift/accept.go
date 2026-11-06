// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"io"
	"strings"

	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

// runAccept is `downshift accept [--session=<id>] [--status=used|discarded]`:
// Records developer acceptance of the current session's code in the telemetry log.
func runAccept(args []string, out, errw io.Writer) int {
	sessionID := ""
	status := telemetry.AcceptanceUsed

	for _, a := range args {
		switch {
		case strings.HasPrefix(a, "--session="):
			sessionID = strings.TrimPrefix(a, "--session=")
		case a == "--discarded":
			status = telemetry.AcceptanceDiscarded
		case a == "--used":
			status = telemetry.AcceptanceUsed
		case a == "-h" || a == "--help":
			fmt.Fprint(out, acceptUsage)
			return 0
		default:
			fmt.Fprintf(errw, "accept: unknown flag %q\n%s", a, acceptUsage)
			return 2
		}
	}

	if sessionID == "" {
		// Attempt to resolve recent session if not explicitly provided
		events, err := telemetry.ReadEvents()
		if err == nil && len(events) > 0 {
			for i := len(events) - 1; i >= 0; i-- {
				if events[i].SessionID != "" {
					sessionID = events[i].SessionID
					break
				}
			}
		}
	}

	if sessionID == "" {
		fmt.Fprintln(errw, "accept: no active session id found to record acceptance")
		return 1
	}

	if err := telemetry.RecordAcceptance(sessionID, status, envHarness()); err != nil {
		fmt.Fprintf(errw, "accept: failed to record: %v\n", err)
		return 1
	}

	fmt.Fprintf(out, "✓ Recorded code acceptance: session %s marked as %s.\n", sessionID, status)
	return 0
}

const acceptUsage = `usage: downshift accept [--session=<id>] [--used | --discarded]

Record developer acceptance for a session's delivered code into the telemetry log.
  --session=<id>  Session hash to record acceptance for (default: most recent session)
  --used          Mark session outputs as accepted and kept (default)
  --discarded     Mark session outputs as reverted or rewritten
`
