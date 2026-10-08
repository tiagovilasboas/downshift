// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

func writeVerificationEvents(t *testing.T, events ...telemetry.Event) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "events.jsonl")
	for _, ev := range events {
		if err := telemetry.AppendTo(path, ev); err != nil {
			t.Fatal(err)
		}
	}
	return path
}

func usageEvent(tier, verification string, cost float64) telemetry.Event {
	c := cost
	return telemetry.Event{Outcome: telemetry.OutcomeUsage, Tier: tier, Verification: verification, ActualCostUSD: &c}
}

func TestVerificationReport_GroupsByTierAndCountsVerdicts(t *testing.T) {
	path := writeVerificationEvents(t,
		usageEvent("small", "passed", 0.01),
		usageEvent("small", "failed", 0.02),
		usageEvent("small", "none", 0.03),
		usageEvent("frontier", "passed", 0.5),
		telemetry.Event{Outcome: telemetry.OutcomeRewriteEmitted, Tier: "small"},
	)
	rep, err := buildVerificationReport(path)
	if err != nil {
		t.Fatal(err)
	}
	if rep.Usage != 4 || rep.Coverage != 1 {
		t.Fatalf("usage=%d coverage=%v, want 4 and 1", rep.Usage, rep.Coverage)
	}
	if len(rep.Tiers) != 2 || rep.Tiers[0].Tier != "frontier" || rep.Tiers[1].Tier != "small" {
		t.Fatalf("tiers not sorted or wrong: %+v", rep.Tiers)
	}
	small := rep.Tiers[1]
	if small.Passed != 1 || small.Failed != 1 || small.None != 1 || small.Unrecorded != 0 {
		t.Fatalf("small verdicts wrong: %+v", small)
	}
	if small.PricedCalls != 3 || small.CostUSD < 0.0599 || small.CostUSD > 0.0601 {
		t.Fatalf("small cost wrong: %+v", small)
	}
}

func TestVerificationReport_MissingFieldIsUnrecordedNotNone(t *testing.T) {
	path := writeVerificationEvents(t, usageEvent("small", "", 0.01))
	rep, err := buildVerificationReport(path)
	if err != nil {
		t.Fatal(err)
	}
	if rep.Coverage != 0 || rep.Tiers[0].Unrecorded != 1 || rep.Tiers[0].None != 0 {
		t.Fatalf("a missing field must count as unrecorded, not none: %+v", rep)
	}
}

func TestVerificationReport_MissingLogIsEmptyNotError(t *testing.T) {
	rep, err := buildVerificationReport(filepath.Join(t.TempDir(), "absent.jsonl"))
	if err != nil {
		t.Fatalf("missing log must not error: %v", err)
	}
	if rep.Usage != 0 || rep.Coverage != 0 || len(rep.Tiers) != 0 {
		t.Fatalf("want empty report, got %+v", rep)
	}
}

func TestVerificationReport_CLIRejectsBadFlagAndEmitsJSON(t *testing.T) {
	var out, errOut bytes.Buffer
	if code := runVerificationReport([]string{"--bogus"}, &out, &errOut); code != 2 {
		t.Fatalf("bad flag exit = %d, want 2", code)
	}
	if !strings.Contains(errOut.String(), "usage: downshift verification-report") {
		t.Fatalf("missing usage message: %q", errOut.String())
	}

	path := writeVerificationEvents(t, usageEvent("small", "passed", 0.01))
	out.Reset()
	errOut.Reset()
	if code := runVerificationReport([]string{"--events=" + path}, &out, &errOut); code != 0 {
		t.Fatalf("exit = %d, stderr=%q", code, errOut.String())
	}
	var rep verificationReport
	if err := json.Unmarshal(out.Bytes(), &rep); err != nil {
		t.Fatalf("output is not JSON: %v", err)
	}
	if rep.Usage != 1 {
		t.Fatalf("usage = %d, want 1", rep.Usage)
	}
}
