// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/tiagovilasboas/downshift/internal/loop"
	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

const loopUsage = `Usage: downshift loop [--task=<desc>] [--sensor=<cmd>] [--iterations=<n>] [--timeout=<dur>] [--session=<id>] [--json] [-- <sensor-cmd>]`

func runLoop(args []string, out, errw io.Writer) int {
	var cfg loop.Config
	cfg.MaxIterations = 3
	cfg.Timeout = 5 * time.Minute
	cfg.Harness = envHarness()
	jsonOutput := false

	var sensorArgs []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			sensorArgs = args[i+1:]
			break
		}
		switch {
		case strings.HasPrefix(a, "--task="):
			cfg.Task = strings.TrimPrefix(a, "--task=")
		case strings.HasPrefix(a, "--sensor="):
			cfg.SensorCmd = strings.TrimPrefix(a, "--sensor=")
		case strings.HasPrefix(a, "--iterations="):
			if n, err := strconv.Atoi(strings.TrimPrefix(a, "--iterations=")); err == nil && n > 0 {
				cfg.MaxIterations = n
			}
		case strings.HasPrefix(a, "--timeout="):
			if d, err := time.ParseDuration(strings.TrimPrefix(a, "--timeout=")); err == nil && d > 0 {
				cfg.Timeout = d
			}
		case strings.HasPrefix(a, "--session="):
			cfg.SessionID = strings.TrimPrefix(a, "--session=")
		case a == "--json":
			jsonOutput = true
		case a == "-h" || a == "--help":
			if h, ok := dailyHelp["loop"]; ok {
				fmt.Fprint(out, h)
			} else {
				fmt.Fprintln(out, loopUsage)
			}
			return 0
		default:
			fmt.Fprintf(errw, "loop: unknown argument %q\n%s\n", a, loopUsage)
			return 2
		}
	}

	if len(sensorArgs) > 0 {
		cfg.SensorCmd = strings.Join(sensorArgs, " ")
	}
	if cfg.Task == "" {
		cfg.Task = "Engineering task"
	}
	if cfg.SessionID == "" {
		// Use recent session if available, else generate one
		events, err := telemetry.ReadEvents()
		if err == nil && len(events) > 0 {
			for i := len(events) - 1; i >= 0; i-- {
				if events[i].SessionID != "" {
					cfg.SessionID = events[i].SessionID
					break
				}
			}
		}
		if cfg.SessionID == "" {
			cfg.SessionID = fmt.Sprintf("sess_loop_%d", time.Now().Unix())
		}
	}

	runner := loop.NewRunner(cfg)
	res, err := runner.Run(context.Background())
	if err != nil && res.FinalStep != loop.StepFailed {
		fmt.Fprintf(errw, "loop error: %v\n", err)
		return 1
	}

	if jsonOutput {
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		_ = enc.Encode(res)
	} else {
		if res.Success {
			fmt.Fprintf(out, "✓ Autonomous loop completed in %d iteration(s) (%v)\n", res.Iterations, res.Duration.Round(time.Millisecond))
			fmt.Fprintf(out, "  Status:  %s\n", res.FinalStep)
			fmt.Fprintf(out, "  Sensor:  passed\n")
			if res.CascadeActive {
				fmt.Fprintf(out, "  Cascade: auto-upgraded (recovered from prior failure)\n")
			}
			if res.Accepted {
				fmt.Fprintf(out, "  Code:    accepted into telemetry\n")
			}
		} else {
			fmt.Fprintf(errw, "✗ Autonomous loop stopped: %s\n", res.Message)
			fmt.Fprintf(errw, "  Iterations: %d / %d\n", res.Iterations, cfg.MaxIterations)
			fmt.Fprintf(errw, "  Final step: %s\n", res.FinalStep)
			return 1
		}
	}

	return 0
}
