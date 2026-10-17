// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/tiagovilasboas/downshift/internal/core"
	"github.com/tiagovilasboas/downshift/internal/routingv2/training"
)

// runFeedback records engineer-reviewed results without coupling the loop to
// any harness completion protocol.
func runFeedback(args []string) int {
	if len(args) > 0 && args[0] == "stats" {
		if len(args) != 1 {
			fmt.Fprintln(os.Stderr, "usage: downshift feedback stats")
			return 2
		}
		store := training.NewEventStore(training.DefaultEventsPath())
		events, err := store.Load()
		if err != nil {
			fmt.Fprintf(os.Stderr, "error reading loop events: %v\n", err)
			return 1
		}
		type counts struct{ pending, success, retry, failed, trainable int }
		byHarness := make(map[string]counts)
		for _, event := range events {
			c := byHarness[event.Harness]
			if event.Outcome != nil && event.Outcome.RequiredTier != nil {
				c.trainable++
			}
			if event.Outcome == nil {
				c.pending++
			} else {
				switch {
				case event.Outcome.Success:
					c.success++
				case event.Outcome.Retry:
					c.retry++
				case event.Outcome.Failed:
					c.failed++
				}
			}
			byHarness[event.Harness] = c
		}
		if len(events) == 0 {
			fmt.Fprintln(os.Stdout, "No routing events yet.")
			return 0
		}
		fmt.Fprintf(os.Stdout, "%-16s %8s %8s %8s %8s %10s\n", "Harness", "Success", "Retry", "Failed", "Pending", "Trainable")
		var names []string
		for name := range byHarness {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			c := byHarness[name]
			fmt.Fprintf(os.Stdout, "%-16s %8d %8d %8d %8d %10d\n", name, c.success, c.retry, c.failed, c.pending, c.trainable)
		}
		return 0
	}
	if len(args) == 0 || args[0] == "list" {
		pendingOnly := len(args) == 2 && args[1] == "--pending"
		if len(args) > 2 || (len(args) == 2 && !pendingOnly) {
			fmt.Fprintln(os.Stderr, "usage: downshift feedback list [--pending] | stats")
			return 2
		}
		store := training.NewEventStore(training.DefaultEventsPath())
		events, err := store.Load()
		if err != nil {
			fmt.Fprintf(os.Stderr, "error reading loop events: %v\n", err)
			return 1
		}
		if len(events) == 0 {
			fmt.Fprintln(os.Stdout, "No routing events yet. Run a subagent task through a configured harness first.")
			return 0
		}
		shown := 0
		for i := len(events) - 1; i >= 0 && shown < 50; i-- {
			event := events[i]
			if pendingOnly && event.Outcome != nil {
				continue
			}
			outcome := "pending"
			if event.Outcome != nil {
				switch {
				case event.Outcome.Success:
					outcome = "success"
				case event.Outcome.Retry:
					outcome = "retry → " + event.Outcome.RetryTier.String()
				case event.Outcome.Failed:
					outcome = "failed"
				}
			}
			fmt.Fprintf(os.Stdout, "%s  %-10s %-8s %-14s %s\n", event.ID, event.Harness, event.SelectedTier, outcome, event.Timestamp.Format(time.RFC3339))
			shown++
		}
		return 0
	}
	if len(args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: downshift feedback list [--pending] | stats | <id> <success|retry|failed> [--retry-tier=MID|FRONTIER] [--required-tier=SMALL|MID|FRONTIER]")
		return 2
	}
	outcome := training.Outcome{}
	switch args[1] {
	case "success":
		outcome.Success = true
	case "retry":
		outcome.Retry = true
	case "failed":
		outcome.Failed = true
	default:
		fmt.Fprintf(os.Stderr, "unknown outcome %q; use success, retry, or failed\n", args[1])
		return 2
	}
	for _, arg := range args[2:] {
		if strings.HasPrefix(arg, "--retry-tier=") {
			switch strings.ToUpper(strings.TrimPrefix(arg, "--retry-tier=")) {
			case "MID":
				outcome.RetryTier = core.TierMid
			case "FRONTIER":
				outcome.RetryTier = core.TierFrontier
			default:
				fmt.Fprintln(os.Stderr, "retry tier must be MID or FRONTIER")
				return 2
			}
		} else if strings.HasPrefix(arg, "--required-tier=") {
			var required core.Tier
			switch strings.ToUpper(strings.TrimPrefix(arg, "--required-tier=")) {
			case "SMALL":
				required = core.TierSmall
			case "MID":
				required = core.TierMid
			case "FRONTIER":
				required = core.TierFrontier
			default:
				fmt.Fprintln(os.Stderr, "required tier must be SMALL, MID, or FRONTIER")
				return 2
			}
			outcome.RequiredTier = &required
		} else {
			fmt.Fprintf(os.Stderr, "unknown feedback option %q\n", arg)
			return 2
		}
	}
	store := training.NewEventStore(training.DefaultEventsPath())
	if err := store.AddOutcome(args[0], outcome); err != nil {
		fmt.Fprintf(os.Stderr, "error recording feedback: %v\n", err)
		return 1
	}
	fmt.Fprintf(os.Stdout, "Feedback recorded for %s. Train a candidate with `downshift train --from-events --output=candidate.json`.\n", args[0])
	return 0
}
