// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/tiagovilasboas/downshift/internal/paths"
	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

// verificationTierReport is one tier's share of usage events.
type verificationTierReport struct {
	Tier        string  `json:"tier"`
	Usage       int     `json:"usage_events"`
	Passed      int     `json:"passed"`
	Failed      int     `json:"failed"`
	None        int     `json:"none"`
	Unrecorded  int     `json:"unrecorded"`
	CostUSD     float64 `json:"cost_usd"`
	PricedCalls int     `json:"priced_calls"`
}

// verificationReport answers whether subagents verified their own work, per
// tier, so cost can be read next to a check that actually ran. Coverage counts
// events written before the verification field existed as unrecorded, never as
// none: a missing field is not evidence that no check ran.
type verificationReport struct {
	Source   string                   `json:"source"`
	Usage    int                      `json:"usage_events"`
	Coverage float64                  `json:"verification_coverage"`
	Tiers    []verificationTierReport `json:"tiers"`
}

func runVerificationReport(args []string, out io.Writer, errOut io.Writer) int {
	path := defaultVerificationEventsPath()
	for _, arg := range args {
		value, ok := strings.CutPrefix(arg, "--events=")
		if !ok || strings.TrimSpace(value) == "" {
			fmt.Fprintln(errOut, "usage: downshift verification-report [--events=<events.jsonl>] (JSON output)")
			return 2
		}
		path = value
	}
	rep, err := buildVerificationReport(path)
	if err != nil {
		fmt.Fprintf(errOut, "error reading events: %v\n", err)
		return 1
	}
	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	if err := enc.Encode(rep); err != nil {
		return 1
	}
	return 0
}

// defaultVerificationEventsPath mirrors the telemetry default: the env override
// first, then the state directory.
func defaultVerificationEventsPath() string {
	if p := os.Getenv("DOWNSHIFT_EVENT_LOG"); p != "" {
		return p
	}
	p, err := paths.EventsPath()
	if err != nil {
		return "events.jsonl"
	}
	return p
}

func buildVerificationReport(path string) (verificationReport, error) {
	rep := verificationReport{Source: path}
	events, err := telemetry.ReadEventsFrom(path)
	if err != nil {
		return rep, err
	}

	byTier := map[string]*verificationTierReport{}
	withField := 0
	for _, ev := range events {
		if ev.Outcome != telemetry.OutcomeUsage {
			continue
		}
		rep.Usage++
		tier := strings.ToLower(ev.Tier)
		if tier == "" {
			tier = "unknown"
		}
		t := byTier[tier]
		if t == nil {
			t = &verificationTierReport{Tier: tier}
			byTier[tier] = t
		}
		t.Usage++
		switch ev.Verification {
		case "passed":
			t.Passed++
			withField++
		case "failed":
			t.Failed++
			withField++
		case "none":
			t.None++
			withField++
		default:
			t.Unrecorded++
		}
		if ev.ActualCostUSD != nil {
			t.CostUSD += *ev.ActualCostUSD
			t.PricedCalls++
		}
	}
	if rep.Usage > 0 {
		rep.Coverage = float64(withField) / float64(rep.Usage)
	}
	for _, t := range byTier {
		rep.Tiers = append(rep.Tiers, *t)
	}
	sort.Slice(rep.Tiers, func(i, j int) bool { return rep.Tiers[i].Tier < rep.Tiers[j].Tier })
	return rep, nil
}
