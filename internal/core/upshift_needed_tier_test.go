// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package core_test

import (
	"path/filepath"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/core"
)

func upshiftTestEnv(t *testing.T) {
	t.Helper()
	t.Setenv("DOWNSHIFT_MINILM", "0")
	t.Setenv("DOWNSHIFT_EXPLICIT_UPSHIFT", "")
	t.Setenv("DOWNSHIFT_SESSION_MODELS", filepath.Join(t.TempDir(), "absent.json"))
}

// A confident upshift writes the least session model at the needed tier, the
// same model a downshift of the same task lands on.
func TestUpshift_WritesLeastModelAtNeededTier(t *testing.T) {
	upshiftTestEnv(t)
	sess := core.KnownSession([]string{"claude-haiku-4-5", "claude-sonnet-5-5", "claude-opus-5-5", "claude-fable-5-1"})
	cases := []struct{ name, prompt, current, want string }{
		{"medium from haiku", "implement the feature to export sales as CSV", "claude-haiku-4-5", "claude-sonnet-5-5"},
		{"medium from opus (same task, downshift)", "implement the feature to export sales as CSV", "claude-opus-5-5", "claude-sonnet-5-5"},
		{"complex from haiku", "rearchitect the auth module to support multi-tenant organisations", "claude-haiku-4-5", "claude-fable-5-1"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := core.Route(tc.prompt, "claude-code", tc.current, cat)
			d.RequestedID = tc.current
			if !d.Confident {
				t.Fatalf("precondition: want a confident decision, got %+v", d)
			}
			plan := d.PlanForSession(core.ClaudeCodeCaps, cat, sess)
			if !plan.RewriteModel || plan.Model.ID != tc.want {
				t.Fatalf("verdict %s tier %s: plan %q (rewrite %v), want %q", d.Verdict, d.Tier, plan.Model.ID, plan.RewriteModel, tc.want)
			}
		})
	}
}

func TestUpshift_NeededTierMissingOrExhausted(t *testing.T) {
	upshiftTestEnv(t)
	d := core.Decision{
		Harness: "claude-code", Verdict: core.VerdictUpshift, Confident: true, Tier: core.TierMid,
		CurrentModel: core.Model{ID: "claude-haiku-4-5", Harness: "claude-code", Tier: core.TierSmall},
		RequestedID:  "claude-haiku-4-5",
	}
	noMid := core.KnownSession([]string{"claude-haiku-4-5", "claude-opus-5-5"})
	if plan := d.PlanForSession(core.ClaudeCodeCaps, cat, noMid); !plan.RewriteModel || plan.Model.ID != "claude-opus-5-5" {
		t.Fatalf("no mid model: plan %+v, want opus", plan)
	}
	midOut := core.KnownSession([]string{"claude-haiku-4-5", "claude-sonnet-5-5", "claude-opus-5-5"})
	midOut.Exhausted = []string{"claude-sonnet-5-5"}
	if plan := d.PlanForSession(core.ClaudeCodeCaps, cat, midOut); !plan.RewriteModel || plan.Model.ID != "claude-opus-5-5" {
		t.Fatalf("mid exhausted: plan %+v, want opus", plan)
	}
}

func TestUpshift_GuessedBaselineOrExplicitKeepsStrongest(t *testing.T) {
	upshiftTestEnv(t)
	sess := core.KnownSession([]string{"claude-haiku-4-5", "claude-sonnet-5-5", "claude-opus-5-5"})
	route := func() core.Decision {
		d := core.Route("implement the feature to export sales as CSV", "claude-code", "claude-haiku-4-5", cat)
		d.RequestedID = "claude-haiku-4-5"
		return d
	}
	d := route()
	d.BaselineGuessed = true
	if plan := d.PlanForSession(core.ClaudeCodeCaps, cat, sess); !plan.RewriteModel || plan.Model.ID != "claude-opus-5-5" {
		t.Fatalf("guessed baseline: plan %+v, want opus", plan)
	}
	t.Setenv("DOWNSHIFT_EXPLICIT_UPSHIFT", "1")
	if plan := route().PlanForSession(core.ClaudeCodeCaps, cat, sess); !plan.RewriteModel || plan.Model.ID != "claude-opus-5-5" {
		t.Fatalf("explicit_upshift: plan %+v, want opus", plan)
	}
}
