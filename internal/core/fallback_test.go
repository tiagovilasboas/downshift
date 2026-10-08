// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package core_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/core"
)

func TestMain(m *testing.M) {
	os.Setenv("DOWNSHIFT_SESSION_MODELS", filepath.Join(os.TempDir(), "downshift-session-models-absent.json"))
	os.Setenv("DOWNSHIFT_DISCOVERY", "off")
	os.Exit(m.Run())
}

// TestPlanForSession_OnlySessionIDs covers every harness with the same rule:
// the emitted id is a session member, a catalog-only id is never emitted, and
// a missing session list does not rewrite.
func TestPlanForSession_OnlySessionIDs(t *testing.T) {
	cases := []struct {
		harness      string
		caps         core.HarnessCapabilities
		frontier     string
		catalogSmall string
		session      []string
		want         string
	}{
		{
			harness: "cursor", caps: core.CursorCaps,
			frontier: "claude-opus-5-thinking-high", catalogSmall: "claude-4.5-haiku-thinking",
			session: []string{"claude-4.5-sonnet-thinking", "claude-opus-5-thinking-high"},
			want:    "claude-4.5-sonnet-thinking",
		},
		{
			harness: "claude-code", caps: core.ClaudeCodeCaps,
			frontier: "claude-opus-5-5", catalogSmall: "claude-haiku-4-5",
			session: []string{"claude-sonnet-5-5", "claude-opus-5-5"},
			want:    "claude-sonnet-5-5",
		},
		{
			harness: "codex", caps: core.CodexCaps,
			frontier: "gpt-5.6-sol", catalogSmall: "gpt-5.6-luna",
			session: []string{"gpt-5.6-terra", "gpt-5.6-sol"},
			want:    "gpt-5.6-terra",
		},
	}
	for _, tc := range cases {
		t.Run(tc.harness, func(t *testing.T) {
			d := core.Decision{
				Harness:      tc.harness,
				Verdict:      core.VerdictDownshift,
				Confident:    true,
				Tier:         core.TierSmall,
				Model:        core.Model{ID: tc.catalogSmall, Tier: core.TierSmall, Harness: tc.harness},
				CurrentModel: core.Model{ID: tc.frontier, Tier: core.TierFrontier, Harness: tc.harness},
			}
			plan := d.PlanForSession(tc.caps, cat, core.KnownSession(tc.session))
			if !plan.RewriteModel {
				t.Fatal("expected a rewrite to a session model")
			}
			if plan.Model.ID != tc.want {
				t.Fatalf("model = %q, want %q", plan.Model.ID, tc.want)
			}
			if plan.Model.ID == tc.catalogSmall {
				t.Fatal("emitted the catalog smallest id, which is not in the session")
			}
			for _, id := range tc.session {
				if id == plan.Model.ID {
					return
				}
			}
			t.Fatalf("emitted %q is not in the session", plan.Model.ID)
		})
	}
}

func TestPlanForSession_MissingSessionDoesNotRewrite(t *testing.T) {
	for _, harness := range []string{"cursor", "claude-code"} {
		t.Run(harness, func(t *testing.T) {
			d := core.Decision{
				Harness:      harness,
				Verdict:      core.VerdictDownshift,
				Confident:    true,
				Tier:         core.TierSmall,
				Model:        cat.ModelFor(harness, core.TierSmall),
				CurrentModel: cat.ModelFor(harness, core.TierFrontier),
			}
			plan := d.Plan(core.CursorCaps, cat)
			if plan.RewriteModel {
				t.Fatalf("%s: missing session rewrote to %q", harness, plan.Model.ID)
			}
		})
	}
}

func TestPlan_ExplicitOnlySkipsSessionRouting(t *testing.T) {
	session := core.KnownSession([]string{"gpt-5.6-luna", "gpt-6-astra"})
	d := core.Decision{
		Harness:      "codex",
		Verdict:      core.VerdictDownshift,
		Confident:    true,
		Tier:         core.TierSmall,
		Model:        core.Model{ID: "gpt-5.6-luna", Tier: core.TierSmall, Harness: "codex"},
		CurrentModel: core.Model{ID: "gpt-6-astra", Tier: core.TierFrontier, Harness: "codex"},
	}
	plan := d.PlanForSession(core.CodexCaps, cat, session)
	if !plan.PreserveExplicit {
		t.Fatal("explicit_only current model must be preserved")
	}
	if plan.RewriteModel {
		t.Fatal("explicit_only model must not be rewritten")
	}
}

func TestLoadSessionFile_PerHarness(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "session-models.json")
	body := []byte(`{
		"_comment": "example only",
		"cursor": ["composer-2.5", "claude-4.5-sonnet-thinking"],
		"codex": ["gpt-5.6-luna"]
	}`)
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatal(err)
	}
	got := core.LoadSessionFile("cursor", path)
	if !got.Known || len(got.IDs) != 2 || got.IDs[0] != "composer-2.5" {
		t.Fatalf("cursor session = %+v", got)
	}
	if core.LoadSessionFile("claude-code", path).Known {
		t.Fatal("missing harness key must stay unknown")
	}
	if core.LoadSessionFile("cursor", filepath.Join(dir, "missing.json")).Known {
		t.Fatal("missing file must stay unknown")
	}
}
