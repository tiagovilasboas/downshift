// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/tiagovilasboas/downshift/internal/routingv2/training"
	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

const shadowReportUsage = "usage: downshift shadow-report [--events=<loop-events.jsonl>] [--telemetry=<events.jsonl>] [--text]"

func runShadowReport(args []string) int {
	return shadowReport(args, os.Stdout, os.Stderr)
}

// shadowReport joins the candidate classifier observations (loop events)
// with the E5 shadow and fuse objects on the telemetry log. Both files are
// optional; a missing one contributes nothing. Output is JSON unless --text.
func shadowReport(args []string, stdout, stderr io.Writer) int {
	path := training.DefaultEventsPath()
	telemetryPath := ""
	text := false
	for _, arg := range args {
		switch {
		case arg == "--text":
			text = true
		case strings.HasPrefix(arg, "--events=") && strings.TrimSpace(strings.TrimPrefix(arg, "--events=")) != "":
			path = strings.TrimPrefix(arg, "--events=")
		case strings.HasPrefix(arg, "--telemetry=") && strings.TrimSpace(strings.TrimPrefix(arg, "--telemetry=")) != "":
			telemetryPath = strings.TrimPrefix(arg, "--telemetry=")
		default:
			fmt.Fprintln(stderr, shadowReportUsage)
			return 2
		}
	}
	events, err := training.NewEventStore(path).Load()
	if err != nil {
		fmt.Fprintf(stderr, "error loading shadow events: %v\n", err)
		return 1
	}
	var tel []telemetry.Event
	if telemetryPath == "" {
		tel, err = telemetry.ReadEvents()
	} else {
		tel, err = telemetry.ReadEventsFrom(telemetryPath)
	}
	if err != nil {
		fmt.Fprintf(stderr, "error loading telemetry events: %v\n", err)
		return 1
	}
	report := training.SummarizeShadow(events)
	report.E5Shadow, report.E5Fuse = training.SummarizeE5(tel)
	if text {
		report.WriteText(stdout)
		return 0
	}
	if err := json.NewEncoder(stdout).Encode(report); err != nil {
		return 1
	}
	return 0
}
