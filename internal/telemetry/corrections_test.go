// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package telemetry_test

import (
	"strings"
	"testing"
	"time"

	"github.com/tiagovilasboas/downshift/internal/core"
	"github.com/tiagovilasboas/downshift/internal/telemetry"
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
	// A held decision never changed the spawn: it is not a downshift and
	// saves nothing. It is reported separately as not applied.
	if s.Downshifted != 1 {
		t.Errorf("Downshifted = %d, want 1 (held decision is not applied)", s.Downshifted)
	}
	if s.NotApplied != 1 {
		t.Errorf("NotApplied = %d, want 1", s.NotApplied)
	}
	if s.NormRouted < 1.19 || s.NormRouted > 1.21 {
		t.Errorf("NormRouted = %.3f, want 1.2 (only the applied downshift saves)", s.NormRouted)
	}
}

func TestAggregate_AllowOutcomeIsNotSavings(t *testing.T) {
	allowed := makeEvent("cc", "MEDIUM", "DOWNSHIFT", 0.4)
	allowed.Outcome = telemetry.OutcomeAllow
	s := telemetry.Aggregate([]telemetry.Event{allowed})
	if s.Downshifted != 0 || s.NotApplied != 1 {
		t.Fatalf("Downshifted=%d NotApplied=%d, want 0/1", s.Downshifted, s.NotApplied)
	}
	if saved, _ := s.NormSaved(); saved != 0 {
		t.Fatalf("allow event must not save units, got %.3f", saved)
	}
	sum := telemetry.ExportSummary([]telemetry.Event{allowed}, 0, time.Now())
	if sum.DownshiftRate != 0 || sum.NotApplied != 1 {
		t.Fatalf("export downshift_rate=%.2f not_applied=%d, want 0/1", sum.DownshiftRate, sum.NotApplied)
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
