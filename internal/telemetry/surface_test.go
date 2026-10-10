// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package telemetry_test

import (
	"bytes"
	"math"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/core"
	"github.com/tiagovilasboas/downshift/internal/hookport"
	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

func TestSpawnSurfaceHarnessFixtures(t *testing.T) {
	t.Run("port with honor and usage", func(t *testing.T) {
		const decisionID = "dec-claude"
		var recorded []telemetry.Event
		port := hookport.Port{
			ID: "claude-code",
			Honor: func(raw []byte, binaryVersion string, r core.Resolver) (any, string) {
				recorded = append(recorded, telemetry.Event{
					Harness:        "claude-code",
					Outcome:        telemetry.OutcomeResolved,
					LinkedDecision: decisionID,
					RewriteHonored: boolPtr(true),
				})
				return map[string]string{"model": "small"}, "honored"
			},
			Usage: func(raw []byte, binaryVersion string, r core.Resolver) string {
				recorded = append(recorded, telemetry.Event{
					Harness:        "claude-code",
					Outcome:        telemetry.OutcomeUsage,
					LinkedDecision: decisionID,
					InputTokens:    i64Ptr(120),
					OutputTokens:   i64Ptr(30),
				})
				return "usage"
			},
		}

		honorObs := hookport.ObserveHonor(port, []byte(`{"child":true}`), "v-test", nil)
		if !honorObs.Observed {
			t.Fatal("Honor func ran but ObserveHonor reported unobserved")
		}
		usageObs := hookport.ObserveUsage(port, []byte(`{"usage":true}`), "v-test", nil)
		if !usageObs.Observed {
			t.Fatal("Usage func ran but ObserveUsage reported unobserved")
		}

		decision := appliedDownshift("claude-code", decisionID, "available", false, 0.4)
		events := append([]telemetry.Event{decision}, recorded...)
		assertSurface(t, events, decision, telemetry.HonorObserved, telemetry.QuotaAvailable, telemetry.UsageObserved)

		stats := telemetry.Aggregate(events)
		if stats.HonorObserved != 1 || stats.UsageObserved != 1 {
			t.Fatalf("aggregate honor=%d usage=%d, want 1 and 1", stats.HonorObserved, stats.UsageObserved)
		}
		saved, _ := stats.NormSaved()
		if math.Abs(saved-0.4) > 1e-9 {
			t.Fatalf("NormSaved() = %v, want 0.4", saved)
		}
	})

	t.Run("nil honor and usage stay unobserved", func(t *testing.T) {
		for _, id := range []string{"cursor", "codex", "antigravity", "kirocrew", "grok"} {
			port := hookport.Port{ID: id}
			if hookport.ObserveHonor(port, []byte(`{}`), "v-test", nil).Observed {
				t.Fatalf("%s nil Honor was observed", id)
			}
			if hookport.ObserveUsage(port, []byte(`{}`), "v-test", nil).Observed {
				t.Fatalf("%s nil Usage was observed", id)
			}
		}

		const (
			decisionID = "dec-cursor"
			sessionID  = "sess-cursor"
			written    = "model-small"
		)
		decision := appliedDownshift("cursor", decisionID, "unknown", false, 0.55)
		decision.SessionID = sessionID
		decision.FromModel = "model-frontier"
		decision.ToModel = written

		// A later spawn that repeats the written model is not a link.
		later := telemetry.Event{
			CorrelationID: "later-cursor",
			Harness:       "cursor",
			SessionID:     sessionID,
			Outcome:       telemetry.OutcomeAllow,
			Verdict:       "OK",
			FromModel:     written,
			ToModel:       written,
		}
		// rewrite_emitted is not honor, even with rewrite_honored set.
		emitted := telemetry.Event{
			CorrelationID:    "emit-cursor",
			Harness:          "cursor",
			Outcome:          telemetry.OutcomeRewriteEmitted,
			Verdict:          "DOWNSHIFT",
			LinkedDecision:   decisionID,
			RewriteHonored:   boolPtr(true),
			EstimatedSavings: 0.25,
			QuotaStatus:      "unknown",
			FromModel:        "model-frontier",
			ToModel:          written,
		}
		// Catalog estimated_savings without token counts is not usage.
		catalog := telemetry.Event{
			Harness:          "cursor",
			Outcome:          telemetry.OutcomeUsage,
			LinkedDecision:   decisionID,
			EstimatedSavings: 0.9,
		}
		resolvedFalse := telemetry.Event{
			Harness:        "cursor",
			Outcome:        telemetry.OutcomeResolved,
			LinkedDecision: decisionID,
			RewriteHonored: boolPtr(false),
		}
		held := appliedDownshift("cursor", "held-cursor", "available", true, 0.7)

		events := []telemetry.Event{decision, later, emitted, catalog, resolvedFalse, held}
		assertSurface(t, events, decision, telemetry.HonorUnobserved, telemetry.QuotaUnknown, telemetry.UsageUnobserved)

		honor, usage := telemetry.ObservedLinks(events)
		if honor[telemetry.SurfaceKey(decision)] || usage[telemetry.SurfaceKey(decision)] {
			t.Fatalf("nil harness links honor=%v usage=%v, want both false", honor[telemetry.SurfaceKey(decision)], usage[telemetry.SurfaceKey(decision)])
		}

		shifted, inferred := telemetry.CountInferredHonored(events)
		if shifted < 1 || inferred < 1 {
			t.Fatalf("inference fixture shifted=%d honored=%d, want the later spawn to infer", shifted, inferred)
		}

		stats := telemetry.Aggregate(events)
		if stats.HonorObserved != 0 || stats.UsageObserved != 0 {
			t.Fatalf("nil harness aggregate honor=%d usage=%d, want 0 and 0", stats.HonorObserved, stats.UsageObserved)
		}
		saved, _ := stats.NormSaved()
		if saved != 0 {
			t.Fatalf("nil harness NormSaved() = %v, want 0", saved)
		}

		var report bytes.Buffer
		telemetry.PrintStats(events, telemetry.StatsOptions{}, &report)
		if !strings.Contains(report.String(), "Honor                 observed 0  unobserved") {
			t.Fatalf("report printed inference as honor:\n%s", report.String())
		}
	})
}

func TestProjectQuotaAndSavingsCredit(t *testing.T) {
	cases := []struct {
		name       string
		status     string
		creditHeld bool
		wantQuota  string
		wantCredit bool
	}{
		{name: "available", status: "available", wantQuota: telemetry.QuotaAvailable, wantCredit: true},
		{name: "available but credit held", status: "available", creditHeld: true, wantQuota: telemetry.QuotaHeld},
		{name: "exhausted", status: "exhausted", wantQuota: telemetry.QuotaHeld},
		{name: "stale", status: "stale", wantQuota: telemetry.QuotaHeld},
		{name: "unknown", status: "unknown", wantQuota: telemetry.QuotaUnknown},
		{name: "empty", status: "", wantQuota: telemetry.QuotaUnknown},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := telemetry.ProjectQuota(tc.status, tc.creditHeld); got != tc.wantQuota {
				t.Fatalf("ProjectQuota(%q, %v) = %q, want %q", tc.status, tc.creditHeld, got, tc.wantQuota)
			}
			ev := appliedDownshift("cursor", "credit-"+tc.name, tc.status, tc.creditHeld, 0.5)
			if got := telemetry.SavingsCredit(ev); got != tc.wantCredit {
				t.Fatalf("SavingsCredit() = %v, want %v", got, tc.wantCredit)
			}
		})
	}
}

func appliedDownshift(harness, id, quota string, creditHeld bool, savings float64) telemetry.Event {
	return telemetry.Event{
		CorrelationID:    id,
		Harness:          harness,
		Verdict:          "DOWNSHIFT",
		Outcome:          telemetry.OutcomeRewriteEmitted,
		EstimatedSavings: savings,
		QuotaStatus:      quota,
		CreditHeld:       creditHeld,
		FromModel:        "model-frontier",
		ToModel:          "model-small",
	}
}

func assertSurface(t *testing.T, events []telemetry.Event, decision telemetry.Event, honor, quota, usage string) {
	t.Helper()
	honored, used := telemetry.ObservedLinks(events)
	key := telemetry.SurfaceKey(decision)
	surface := telemetry.SurfaceFor(decision, honored[key], used[key])
	if surface.Honor != honor || surface.Quota != quota || surface.Usage != usage {
		t.Fatalf("surface = %+v, want honor %s quota %s usage %s", surface, honor, quota, usage)
	}
}

func boolPtr(v bool) *bool { return &v }

func i64Ptr(v int64) *int64 { return &v }
