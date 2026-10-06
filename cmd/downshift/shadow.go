// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/tiagovilasboas/downshift/internal/routingv2/training"
)

func runShadowReport(args []string) int {
	path := training.DefaultEventsPath()
	for _, arg := range args {
		if !strings.HasPrefix(arg, "--events=") || strings.TrimSpace(strings.TrimPrefix(arg, "--events=")) == "" {
			fmt.Fprintln(os.Stderr, "usage: downshift shadow-report [--events=<loop-events.jsonl>] (JSON output)")
			return 2
		}
		path = strings.TrimPrefix(arg, "--events=")
	}
	events, err := training.NewEventStore(path).Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error loading shadow events: %v\n", err)
		return 1
	}
	if err := json.NewEncoder(os.Stdout).Encode(training.SummarizeShadow(events)); err != nil {
		return 1
	}
	return 0
}
