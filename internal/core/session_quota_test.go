// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: BUSL-1.1

package core_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/tiagovilasboas/harness-downshift/internal/core"
)

func TestHarnessOwnsID_RejectsOtherHarnessSlug(t *testing.T) {
	session := core.KnownSession([]string{"claude-haiku-4-5", "claude-sonnet-4-6", "claude-opus-4-8"})
	if core.HarnessOwnsID("claude-code", "composer-2.5", session, cat) {
		t.Fatal("composer-2.5 is a cursor session id, not a claude-code id")
	}
	if core.HarnessOwnsID("claude-code", "grok-4.7-xhigh", session, cat) {
		t.Fatal("family prefix must not own grok-4.7-xhigh on claude-code")
	}
	if !core.HarnessOwnsID("claude-code", "claude-opus-4-8", session, cat) {
		t.Fatal("claude-opus-4-8 is an exact claude-code id")
	}
}

func TestPlanForSession_ForeignIDDoesNotRewrite(t *testing.T) {
	session := core.KnownSession([]string{"claude-haiku-4-5", "claude-sonnet-4-6", "claude-opus-4-8"})
	d := core.Decision{
		Harness:      "claude-code",
		RequestedID:  "composer-2.5",
		Verdict:      core.VerdictDownshift,
		Confident:    true,
		Tier:         core.TierSmall,
		Model:        core.Model{ID: "claude-haiku-4-5", Tier: core.TierSmall, Harness: "claude-code"},
		CurrentModel: core.Model{ID: "", Harness: "claude-code"},
	}
	plan := d.PlanForSession(core.ClaudeCodeCaps, cat, session)
	if !plan.HoldForeign || plan.RewriteModel {
		t.Fatalf("plan = %+v, want hold without rewrite", plan)
	}
}

func TestPlanForSession_PrefersIncludedAndSkipsExhausted(t *testing.T) {
	session := core.KnownSession([]string{
		"claude-4.5-haiku-thinking",
		"composer-2.5",
		"claude-opus-5-thinking-high",
		"inherit",
	})
	session.Included = []string{"composer-2.5"}
	session.Exhausted = []string{"claude-4.5-haiku-thinking"}
	d := core.Decision{
		Harness:     "cursor",
		RequestedID: "claude-opus-5-thinking-high",
		Verdict:     core.VerdictDownshift,
		Confident:   true,
		Checked:     true,
		SafeVerdict: core.VerdictDownshift,
		Tier:        core.TierSmall,
		Model:       core.Model{ID: "claude-4.5-haiku-thinking", Tier: core.TierSmall, Harness: "cursor"},
		CurrentModel: core.Model{
			ID: "claude-opus-5-thinking-high", Tier: core.TierFrontier, Harness: "cursor",
		},
	}
	plan := d.PlanForSession(core.CursorCaps, cat, session)
	if !plan.RewriteModel {
		t.Fatal("expected a rewrite to a session model that still has budget")
	}
	if plan.Model.ID != "composer-2.5" {
		t.Fatalf("model = %q, want composer-2.5", plan.Model.ID)
	}
	if plan.Model.ID == "inherit" || plan.Model.ID == "claude-4.5-haiku-thinking" {
		t.Fatalf("selected %q", plan.Model.ID)
	}
}

func TestCanWriteCatalogID_OnlyOwnCanonical(t *testing.T) {
	cursorSession := core.KnownSession([]string{"composer-2.5", "claude-opus-4-8", "claude-haiku-4-5"})
	if !core.CanWriteCatalogID("cursor", "composer-2.5", cursorSession, cat) {
		t.Fatal("composer-2.5 is a cursor canonical id")
	}
	if core.CanWriteCatalogID("cursor", "claude-opus-4-8", cursorSession, cat) {
		t.Fatal("cursor must not write a claude-code canonical id")
	}
	if core.CanWriteCatalogID("cursor", "claude-haiku-4-5", cursorSession, cat) {
		t.Fatal("cursor must not write an id that is canonical only for another harness")
	}
	claude := core.KnownSession([]string{"claude-haiku-4-5", "composer-2.5"})
	if !core.CanWriteCatalogID("claude-code", "claude-haiku-4-5", claude, cat) {
		t.Fatal("claude-haiku-4-5 is claude-code's canonical id")
	}
	if core.CanWriteCatalogID("claude-code", "composer-2.5", claude, cat) {
		t.Fatal("claude-code must not write a cursor canonical id")
	}
	kiro := core.KnownSession([]string{"claude-haiku-4-5"})
	if !core.CanWriteCatalogID("kirocrew", "claude-haiku-4-5", kiro, cat) {
		t.Fatal("the same canonical string is writable when that harness owns the row")
	}
}

func TestLoadSessionFile_Quota(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "session-models.json")
	body := []byte(`{
		"cursor": ["composer-2.5", "claude-sonnet-5-5-high"],
		"quota": {
			"cursor": {
				"included": ["composer-2.5"],
				"exhausted": ["claude-sonnet-5-5-high"]
			}
		}
	}`)
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatal(err)
	}
	got := core.LoadSessionFile("cursor", path)
	if !got.IsIncluded("composer-2.5") || !got.Blocks("claude-sonnet-5-5-high") {
		t.Fatalf("quota = %+v", got)
	}
	if core.LoadSessionFile("codex", path).Known {
		t.Fatal("quota object must not become a codex allowlist")
	}
}
