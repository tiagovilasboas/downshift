// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: BUSL-1.1
// Commercial use requires a licence — see LICENSE for terms.

package telemetry_test

import (
	"strings"
	"testing"

	"github.com/tiagovilasboas/harness-downshift/internal/core"
	"github.com/tiagovilasboas/harness-downshift/internal/telemetry"
)

func TestFromDecision_OmitsCorrectionFieldsWhenClean(t *testing.T) {
	d := core.Decision{Verdict: core.VerdictDownshift, SafeVerdict: core.VerdictDownshift, Checked: true}
	ev := telemetry.FromDecision(d, telemetry.NewCorrelationID(), "test")
	if ev.SafeVerdict != "" || ev.Corrections != nil {
		t.Errorf("clean decision must omit correction fields, got %+v", ev)
	}
}

func TestFromDecision_RecordsHeldDecision(t *testing.T) {
	d := core.Decision{
		Verdict:     core.VerdictDownshift,
		SafeVerdict: core.VerdictOK,
		Corrections: []string{core.RuleUnconfidentDowngrade},
		Checked:     true,
	}
	ev := telemetry.FromDecision(d, telemetry.NewCorrelationID(), "test")
	if ev.Verdict != "DOWNSHIFT" {
		t.Errorf("Verdict = %q, want classified DOWNSHIFT", ev.Verdict)
	}
	if ev.SafeVerdict != "OK" {
		t.Errorf("SafeVerdict = %q, want OK", ev.SafeVerdict)
	}
}

func TestAggregate_CountsCorrected(t *testing.T) {
	clean := makeEvent("cc", "TRIVIAL", "DOWNSHIFT", 0.8)
	held := makeEvent("cc", "TRIVIAL", "DOWNSHIFT", 0.8)
	held.Corrections = []string{core.RuleUnconfidentDowngrade}
	held.SafeVerdict = "OK"
	s := telemetry.Aggregate([]telemetry.Event{clean, held})
	if s.Corrected != 1 {
		t.Errorf("Corrected = %d, want 1", s.Corrected)
	}
	if s.Downshifted != 2 {
		t.Errorf("Downshifted = %d, want 2 (verdict stands)", s.Downshifted)
	}
}

func TestPrintStats_ShowsSafetyHeld(t *testing.T) {
	held := makeEvent("cc", "TRIVIAL", "DOWNSHIFT", 0.8)
	held.Corrections = []string{core.RuleUnconfidentDowngrade}
	var sb strings.Builder
	telemetry.PrintStats([]telemetry.Event{held}, telemetry.StatsOptions{}, &sb)
	if !strings.Contains(sb.String(), "Safety-held") {
		t.Errorf("stats output must mention safety-held decisions, got:\n%s", sb.String())
	}
}
