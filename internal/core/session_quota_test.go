// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package core_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/core"
)

func TestHarnessOwnsID_RejectsOtherHarnessSlug(t *testing.T) {
	session := core.KnownSession([]string{"claude-haiku-4-5", "claude-sonnet-5-5", "claude-opus-5-5"})
	if core.HarnessOwnsID("claude-code", "composer-2.5", session, cat) {
		t.Fatal("composer-2.5 is a cursor session id, not a claude-code id")
	}
	if core.HarnessOwnsID("claude-code", "grok-4.7-xhigh", session, cat) {
		t.Fatal("family prefix must not own grok-4.7-xhigh on claude-code")
	}
	if !core.HarnessOwnsID("claude-code", "claude-opus-5-5", session, cat) {
		t.Fatal("claude-opus-5-5 is an exact claude-code id")
	}
}

func TestPlanForSession_ForeignIDDoesNotRewrite(t *testing.T) {
	session := core.KnownSession([]string{"claude-haiku-4-5", "claude-sonnet-5-5", "claude-opus-5-5"})
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
	session.CreditsReported = true
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

func TestPlanForSession_CreditsStayInsideThatHarness(t *testing.T) {
	cursor := core.KnownSession([]string{"composer-2.5-fast", "composer-2.5", "grok-4.7-xhigh"})
	cursor.CreditsReported = true
	cursor.Included = []string{"composer-2.5", "grok-4.7-xhigh"}
	up := core.Decision{
		Harness:      "cursor",
		RequestedID:  "composer-2.5-fast",
		Verdict:      core.VerdictUpshift,
		Confident:    true,
		Tier:         core.TierFrontier,
		Model:        core.Model{ID: "grok-4.7-xhigh", Harness: "cursor"},
		CurrentModel: core.Model{ID: "composer-2.5-fast", Harness: "cursor"},
	}
	plan := up.PlanForSession(core.CursorCaps, cat, cursor)
	if plan.Model.ID != "grok-4.7-xhigh" {
		t.Fatalf("cursor upshift = %q, want the last credited session id", plan.Model.ID)
	}

	codex := core.KnownSession([]string{"gpt-6-luna", "gpt-6-sol"})
	codex.CreditsReported = true
	codex.Included = []string{"gpt-6-luna"}
	codexUp := core.Decision{
		Harness:      "codex",
		RequestedID:  "gpt-6-luna",
		Verdict:      core.VerdictUpshift,
		Confident:    true,
		Tier:         core.TierFrontier,
		Model:        core.Model{ID: "gpt-6-luna", Harness: "codex"},
		CurrentModel: core.Model{ID: "gpt-6-sol", Harness: "codex"},
	}
	codexPlan := codexUp.PlanForSession(core.CodexCaps, cat, codex)
	if codexPlan.Model.ID != "gpt-6-luna" {
		t.Fatalf("codex upshift = %q, want its own credit set, not cursor's", codexPlan.Model.ID)
	}
}

func TestCanWriteSessionID_RequiresExactSessionMembership(t *testing.T) {
	cursorSession := core.KnownSession([]string{"composer-2.5", "claude-opus-5-5", "claude-haiku-4-5"})
	if !core.CanWriteSessionID("cursor", "composer-2.5", cursorSession, cat) {
		t.Fatal("composer-2.5 is a cursor canonical id")
	}
	if !core.CanWriteSessionID("cursor", "claude-opus-5-5", cursorSession, cat) {
		t.Fatal("an exact session member must remain usable without catalog ownership")
	}
	if !core.CanWriteSessionID("cursor", "claude-haiku-4-5", cursorSession, cat) {
		t.Fatal("an exact session member must remain usable without catalog ownership")
	}
	claude := core.KnownSession([]string{"claude-haiku-4-5", "composer-2.5"})
	if !core.CanWriteSessionID("claude-code", "claude-haiku-4-5", claude, cat) {
		t.Fatal("claude-haiku-4-5 is claude-code's canonical id")
	}
	if !core.CanWriteSessionID("claude-code", "composer-2.5", claude, cat) {
		t.Fatal("an exact session member must remain usable without catalog ownership")
	}
	kiro := core.KnownSession([]string{"claude-haiku-4-5"})
	if !core.CanWriteSessionID("kirocrew", "claude-haiku-4-5", kiro, cat) {
		t.Fatal("the same canonical string is writable when that harness owns the row")
	}
}

func TestLoadSessionFile_Quota(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "session-models.json")
	body := []byte(`{
		"cursor": ["composer-2.5", "claude-sonnet-5-5-high"],
		"codex": ["gpt-5.6-luna", "gpt-5.6-sol"],
		"quota": {
			"cursor": {
				"included": ["composer-2.5"],
				"exhausted": ["claude-sonnet-5-5-high"]
			},
			"codex": {
				"included": ["gpt-5.6-sol"]
			}
		}
	}`)
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatal(err)
	}
	got := core.LoadSessionFile("cursor", path)
	if !got.Known || !got.IsIncluded("composer-2.5") || !got.Blocks("claude-sonnet-5-5-high") {
		t.Fatalf("file quota = %+v, want this harness's included and exhausted sets", got)
	}
	if got.IsIncluded("gpt-5.6-sol") {
		t.Fatal("cursor file quota must not include an id only codex credited")
	}
	credited := []string{"composer-2.5"}
	exhausted := []string{"claude-sonnet-5-5-high"}
	fromHook := got.WithHookQuota(&credited, &exhausted)
	if !fromHook.CreditsReported || !fromHook.IsIncluded("composer-2.5") || !fromHook.Blocks("claude-sonnet-5-5-high") {
		t.Fatalf("hook quota = %+v", fromHook)
	}
	codex := core.LoadSessionFile("codex", path)
	if !codex.Known || !codex.IsIncluded("gpt-5.6-sol") || codex.IsIncluded("composer-2.5") {
		t.Fatalf("codex quota = %+v, want only codex included ids", codex)
	}
}

func TestUpshift_UncreditedGrokIsNotSelected(t *testing.T) {
	t.Setenv("DOWNSHIFT_EXPLICIT_UPSHIFT", "")
	// The live session list can name grok-4.7-xhigh. The hook credit set does not.
	session := core.KnownSession([]string{"composer-2.5-fast", "composer-2.5", "grok-4.7-xhigh"})
	session.CreditsReported = true
	session.Included = []string{"composer-2.5-fast", "composer-2.5"}
	plan := upshiftPlan("cursor", "composer-2.5-fast", core.CursorCaps, session)
	if !plan.RewriteModel || plan.Model.ID != "composer-2.5" {
		t.Fatalf("plan = %+v, want composer-2.5 inside the reported set", plan)
	}
	if plan.Model.ID == "grok-4.7-xhigh" {
		t.Fatal("grok-4.7-xhigh was selected without being in the hook included set")
	}
}

func TestUpshift_CreditSetsDoNotCrossHarnesses(t *testing.T) {
	t.Setenv("DOWNSHIFT_EXPLICIT_UPSHIFT", "")
	cursorIncluded := []string{"composer-2.5-fast", "composer-2.5"}
	cursor := core.KnownSession([]string{"composer-2.5-fast", "composer-2.5", "gpt-5.6-sol", "grok-4.7-xhigh"}).WithHookQuota(&cursorIncluded, nil)
	cursorPlan := upshiftPlan("cursor", "composer-2.5-fast", core.CursorCaps, cursor)
	if !cursorPlan.RewriteModel || cursorPlan.Model.ID != "composer-2.5" {
		t.Fatalf("cursor plan = %+v, want composer-2.5", cursorPlan)
	}
	if cursorPlan.Model.ID == "gpt-5.6-sol" || cursorPlan.Model.ID == "grok-4.7-xhigh" {
		t.Fatalf("cursor selected %q, which its credit set does not include", cursorPlan.Model.ID)
	}

	codexIncluded := []string{"gpt-5.6-sol"}
	codex := core.KnownSession([]string{"gpt-5.6-sol", "composer-2.5"}).WithHookQuota(&codexIncluded, nil)
	codexPlan := upshiftPlan("codex", "composer-2.5", core.CodexCaps, codex)
	if !codexPlan.RewriteModel || codexPlan.Model.ID != "gpt-5.6-sol" {
		t.Fatalf("codex plan = %+v, want gpt-5.6-sol", codexPlan)
	}
	if codexPlan.Model.ID == "composer-2.5" {
		t.Fatal("codex selected composer-2.5, which only cursor marked as credited")
	}

	claudeIncluded := []string{"claude-haiku-4-5", "claude-sonnet-5-5"}
	claude := core.KnownSession([]string{"claude-haiku-4-5", "claude-sonnet-5-5", "claude-opus-5-5"}).WithHookQuota(&claudeIncluded, nil)
	claudePlan := upshiftPlan("claude-code", "claude-haiku-4-5", core.ClaudeCodeCaps, claude)
	if !claudePlan.RewriteModel || claudePlan.Model.ID != "claude-sonnet-5-5" {
		t.Fatalf("claude-code plan = %+v, want claude-sonnet-5-5", claudePlan)
	}
	if claudePlan.Model.ID == "claude-opus-5-5" {
		t.Fatal("claude-code selected an id outside its own credit set")
	}
}

func TestUpshift_EmptyCreditIntersectionDoesNotRewrite(t *testing.T) {
	session := core.KnownSession([]string{"composer-2.5-fast", "composer-2.5", "grok-4.7-xhigh"})
	session.CreditsReported = true
	session.Included = []string{"gpt-5.6-sol"}
	plan := upshiftPlan("cursor", "composer-2.5", core.CursorCaps, session)
	if plan.RewriteModel || plan.Model.ID != "composer-2.5" {
		t.Fatalf("plan = %+v, want no rewrite when nothing credited is in the session", plan)
	}
}

func TestUpshift_MissingCreditReportUsesSessionMinusExhausted(t *testing.T) {
	t.Setenv("DOWNSHIFT_EXPLICIT_UPSHIFT", "")
	dir := t.TempDir()
	path := filepath.Join(dir, "session-models.json")
	body := []byte(`{
		"codex": ["gpt-5.6-luna", "gpt-5.6-sol", "grok-4.7-xhigh"],
		"quota": {
			"cursor": {"included": ["composer-2.5-fast", "composer-2.5"]}
		}
	}`)
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatal(err)
	}
	codex := core.LoadSessionFile("codex", path)
	if len(codex.Included) != 0 {
		t.Fatalf("codex included = %v, want no invented cursor credits", codex.Included)
	}
	codex.Exhausted = []string{"grok-4.7-xhigh"}
	plan := upshiftPlan("codex", "gpt-5.6-luna", core.CodexCaps, codex)
	if !plan.RewriteModel || plan.Model.ID != "gpt-5.6-sol" {
		t.Fatalf("plan = %+v, want gpt-5.6-sol (session order minus exhausted)", plan)
	}
	if plan.Model.ID == "grok-4.7-xhigh" || plan.Model.ID == "composer-2.5" {
		t.Fatalf("selected %q", plan.Model.ID)
	}
}

func TestExhaustedIDIsNeverSelected(t *testing.T) {
	for _, tc := range []struct {
		harness string
		caps    core.HarnessCapabilities
		ids     []string
		want    string
	}{
		{"cursor", core.CursorCaps, []string{"composer-2.5", "grok-4.7-xhigh"}, "composer-2.5"},
		{"codex", core.CodexCaps, []string{"gpt-5.6-luna", "gpt-5.6-sol"}, "gpt-5.6-luna"},
		{"claude-code", core.ClaudeCodeCaps, []string{"claude-haiku-4-5", "claude-opus-5-5"}, "claude-haiku-4-5"},
	} {
		t.Run(tc.harness, func(t *testing.T) {
			session := core.KnownSession(tc.ids)
			session.Exhausted = []string{tc.ids[len(tc.ids)-1]}
			plan := upshiftPlan(tc.harness, tc.ids[0], tc.caps, session)
			if plan.Model.ID != tc.want {
				t.Fatalf("model = %q, want %q (exhausted %q)", plan.Model.ID, tc.want, tc.ids[len(tc.ids)-1])
			}
			unavailable := []string{tc.ids[len(tc.ids)-1]}
			fromHook := core.KnownSession(tc.ids).WithHookQuota(nil, &unavailable)
			hookPlan := upshiftPlan(tc.harness, tc.ids[0], tc.caps, fromHook)
			if hookPlan.Model.ID != tc.want {
				t.Fatalf("hook unavailable model = %q, want %q", hookPlan.Model.ID, tc.want)
			}
		})
	}
}

func TestExplicitUpshift_DefaultsOff(t *testing.T) {
	t.Setenv("DOWNSHIFT_EXPLICIT_UPSHIFT", "")
	session := core.KnownSession([]string{"composer-2.5", "claude-fable-5-1-thinking-high"})
	plan := upshiftPlan("cursor", "composer-2.5", core.CursorCaps, session)
	if plan.RewriteModel || plan.Model.ID == "claude-fable-5-1-thinking-high" {
		t.Fatalf("plan = %+v, explicit_only must not be an upshift target by default", plan)
	}
	if core.CanWriteSessionID("cursor", "claude-fable-5-1-thinking-high", session, cat) {
		t.Fatal("explicit_only id must not be writable while the flag is off")
	}
	if core.CanWriteSessionID("codex", "gpt-6-astra", core.KnownSession([]string{"gpt-5.6-sol", "gpt-6-astra"}), cat) {
		t.Fatal("gpt-6-astra must not be writable while the flag is off")
	}
}

func TestExplicitUpshift_EnvAndFileLetLastIDWin(t *testing.T) {
	t.Run("env", func(t *testing.T) {
		t.Setenv("DOWNSHIFT_EXPLICIT_UPSHIFT", "1")
		assertExplicitUpshiftSelectsLast(t)
	})
	t.Run("file", func(t *testing.T) {
		t.Setenv("DOWNSHIFT_EXPLICIT_UPSHIFT", "")
		dir := t.TempDir()
		path := filepath.Join(dir, "session-models.json")
		if err := os.WriteFile(path, []byte(`{"explicit_upshift": true, "cursor": ["composer-2.5"]}`), 0o600); err != nil {
			t.Fatal(err)
		}
		t.Setenv("DOWNSHIFT_SESSION_MODELS", path)
		assertExplicitUpshiftSelectsLast(t)
	})
}

func TestExplicitUpshift_DownshiftAndCurrentModelStayPut(t *testing.T) {
	t.Setenv("DOWNSHIFT_EXPLICIT_UPSHIFT", "1")
	session := core.KnownSession([]string{"composer-2.5", "claude-opus-5-thinking-high", "claude-fable-5-1-thinking-high"})
	down := core.Decision{
		Harness:      "cursor",
		RequestedID:  "claude-opus-5-thinking-high",
		Verdict:      core.VerdictDownshift,
		Confident:    true,
		Tier:         core.TierFrontier,
		CurrentModel: core.Model{ID: "claude-opus-5-thinking-high", Harness: "cursor"},
	}
	plan := down.PlanForSession(core.CursorCaps, cat, session)
	if plan.Model.ID == "claude-fable-5-1-thinking-high" || plan.RewriteModel {
		t.Fatalf("downshift plan = %+v, explicit_only must stay out of tier mapping", plan)
	}

	held := core.Decision{
		Harness:      "codex",
		RequestedID:  "gpt-6-astra",
		Verdict:      core.VerdictDownshift,
		Confident:    true,
		Tier:         core.TierSmall,
		Model:        core.Model{ID: "gpt-5.6-luna", Harness: "codex"},
		CurrentModel: core.Model{ID: "gpt-6-astra", Harness: "codex"},
	}
	heldPlan := held.PlanForSession(core.CodexCaps, cat, core.KnownSession([]string{"gpt-5.6-luna", "gpt-6-astra"}))
	if !heldPlan.PreserveExplicit || heldPlan.RewriteModel {
		t.Fatalf("current explicit_only plan = %+v", heldPlan)
	}
}

func TestExplicitUpshift_OutsideCreditSetIsNotSelected(t *testing.T) {
	t.Setenv("DOWNSHIFT_EXPLICIT_UPSHIFT", "1")
	session := core.KnownSession([]string{"composer-2.5", "claude-fable-5-1-thinking-high"})
	session.CreditsReported = true
	session.Included = []string{"composer-2.5"}
	plan := upshiftPlan("cursor", "composer-2.5", core.CursorCaps, session)
	if plan.RewriteModel || plan.Model.ID == "claude-fable-5-1-thinking-high" {
		t.Fatalf("plan = %+v, explicit_only outside the credit set must not be selected", plan)
	}
	if core.CanWriteSessionID("cursor", "claude-fable-5-1-thinking-high", session, cat) {
		t.Fatal("credit set is closed even when explicit upshift is on")
	}
}

func TestClaudeCodeFableIsSelectableWithoutExplicitUpshift(t *testing.T) {
	t.Setenv("DOWNSHIFT_EXPLICIT_UPSHIFT", "")
	if cat.IsExplicitOnly("claude-code", "claude-fable-5-1") {
		t.Fatal("claude-code claude-fable-5-1 is not explicit_only")
	}
	if !cat.IsExplicitOnly("cursor", "claude-fable-5-1-thinking-high") {
		t.Fatal("cursor claude-fable-5-1-thinking-high must stay explicit_only")
	}
	if !cat.IsExplicitOnly("codex", "gpt-6-astra") {
		t.Fatal("codex gpt-6-astra must stay explicit_only")
	}
	session := core.KnownSession([]string{"claude-haiku-4-5", "claude-fable-5-1"})
	plan := upshiftPlan("claude-code", "claude-haiku-4-5", core.ClaudeCodeCaps, session)
	if !plan.RewriteModel || plan.Model.ID != "claude-fable-5-1" {
		t.Fatalf("plan = %+v, want claude-fable-5-1 without the explicit flag", plan)
	}
	if !core.CanWriteSessionID("claude-code", "claude-fable-5-1", session, cat) {
		t.Fatal("claude-code fable is a normal session id")
	}
}

func upshiftPlan(harness, current string, caps core.HarnessCapabilities, session core.SessionList) core.RewritePlan {
	d := core.Decision{
		Harness:      harness,
		RequestedID:  current,
		Verdict:      core.VerdictUpshift,
		Confident:    true,
		Tier:         core.TierFrontier,
		Model:        core.Model{ID: current, Harness: harness},
		CurrentModel: core.Model{ID: current, Harness: harness},
	}
	return d.PlanForSession(caps, cat, session)
}

func assertExplicitUpshiftSelectsLast(t *testing.T) {
	t.Helper()
	cursor := core.KnownSession([]string{"composer-2.5", "claude-fable-5-1-thinking-high"})
	plan := upshiftPlan("cursor", "composer-2.5", core.CursorCaps, cursor)
	if !plan.RewriteModel || plan.Model.ID != "claude-fable-5-1-thinking-high" {
		t.Fatalf("cursor plan = %+v, want the last explicit_only id", plan)
	}
	if !core.CanWriteSessionID("cursor", "claude-fable-5-1-thinking-high", cursor, cat) {
		t.Fatal("flag on must allow writing the cursor explicit_only id")
	}
	codex := core.KnownSession([]string{"gpt-5.6-sol", "gpt-6-astra"})
	codexPlan := upshiftPlan("codex", "gpt-5.6-sol", core.CodexCaps, codex)
	if !codexPlan.RewriteModel || codexPlan.Model.ID != "gpt-6-astra" {
		t.Fatalf("codex plan = %+v, want gpt-6-astra", codexPlan)
	}
	if !core.CanWriteSessionID("codex", "gpt-6-astra", codex, cat) {
		t.Fatal("flag on must allow writing gpt-6-astra")
	}
}
