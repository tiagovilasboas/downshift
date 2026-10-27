// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package cursor_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tiagovilasboas/downshift/internal/adapters/cursor"
	"github.com/tiagovilasboas/downshift/internal/catalog"
	"github.com/tiagovilasboas/downshift/internal/core"
	"github.com/tiagovilasboas/downshift/internal/core/decidertest"
	"github.com/tiagovilasboas/downshift/internal/quota"
)

var cat = catalog.Load()

func TestEventTaskText(t *testing.T) {
	ev := cursor.Event{ToolInput: json.RawMessage(`{"task":"private task"}`)}
	if got := ev.TaskText(); got != "private task" {
		t.Fatalf("TaskText() = %q", got)
	}
}

func catID(tier core.Tier) string {
	return cat.ModelFor("cursor", tier).ID
}

func withCatalogSession(ev cursor.Event) cursor.Event {
	ids := []string{cat.ModelFor("cursor", core.TierSmall).ID, cat.ModelFor("cursor", core.TierMid).ID, cat.ModelFor("cursor", core.TierFrontier).ID}
	ev.SessionModels = &ids
	return ev
}

func decodeUpdated(t *testing.T, out cursor.Output) map[string]any {
	t.Helper()
	if out.UpdatedInput == nil {
		return nil
	}
	var m map[string]any
	if err := json.Unmarshal(out.UpdatedInput, &m); err != nil {
		t.Fatalf("updated_input not valid JSON: %v", err)
	}
	return m
}

func TestHandle_IgnoresNonTaskTools(t *testing.T) {
	ev := cursor.Event{
		ToolName:  "Shell",
		ModelID:   catID(core.TierFrontier),
		ToolInput: json.RawMessage(`{"command":"ls"}`),
	}
	_, note, _ := cursor.Handle(withCatalogSession(ev), cat)
	if note != "" {
		t.Errorf("expected no note for non-Task tool, got %q", note)
	}
}

func TestHandle_FallsBackToPromptField(t *testing.T) {
	decidertest.Use(t, decidertest.Tier(core.TierSmall))
	frontierID := catID(core.TierFrontier)
	ev := cursor.Event{
		ToolName: "Task",
		ModelID:  frontierID,
		ToolInput: json.RawMessage(`{
			"prompt": "fix a typo in the readme",
			"model": "` + frontierID + `"
		}`),
	}
	out, note, _ := cursor.Handle(withCatalogSession(ev), cat)
	if note == "" {
		t.Fatal("expected downshift using prompt field")
	}
	m := decodeUpdated(t, out)
	wantID := catID(core.TierSmall)
	if m["model"] != wantID {
		t.Errorf("model = %v, want %s", m["model"], wantID)
	}
}

func TestHandle_FallsBackToEventModel(t *testing.T) {
	decidertest.Use(t, decidertest.Tier(core.TierSmall))
	frontierID := catID(core.TierFrontier)
	ev := cursor.Event{
		ToolName:  "Task",
		ModelID:   frontierID,
		ToolInput: json.RawMessage(`{"task": "rename the variable"}`),
	}
	out, note, _ := cursor.Handle(withCatalogSession(ev), cat)
	if note == "" {
		t.Fatal("expected downshift using event model_id as current")
	}
	m := decodeUpdated(t, out)
	wantID := catID(core.TierSmall)
	if m["model"] != wantID {
		t.Errorf("model = %v, want %s", m["model"], wantID)
	}
}

// Session candidates, not catalog family aliases, determine every harness target.
func TestHandle_UsesSessionTierForModelFamily(t *testing.T) {
	decidertest.Use(t, decidertest.Tier(core.TierMid))
	// Current opus high, MEDIUM task → midpoint of the ordered session list.
	ev := cursor.Event{
		ToolName: "Task",
		ToolInput: json.RawMessage(`{
			"task": "implement the CSV export feature",
			"model": "claude-opus-5-thinking-high"
		}`),
	}
	out, note, _ := cursor.Handle(withCatalogSession(ev), cat)
	if note == "" {
		t.Fatal("expected a session-target routing note, got none")
	}
	m := decodeUpdated(t, out)
	if m["model"] != cat.ModelFor("cursor", core.TierMid).ID {
		t.Errorf("model = %v, want session mid-tier id %s", m["model"], cat.ModelFor("cursor", core.TierMid).ID)
	}
}

func TestHandle_DoesNotSwitchModelForCatalogFamilyEffort(t *testing.T) {
	// A family/effort variant in the catalog is not a session-tier rewrite.
	ev := cursor.Event{
		ToolName: "Task",
		ToolInput: json.RawMessage(`{
			"task": "rearchitect the payment flow across services with a data migration",
			"model": "claude-opus-5.5-medium"
		}`),
	}
	out, note, _ := cursor.Handle(withCatalogSession(ev), cat)
	if note != "" || out.UpdatedInput != nil {
		t.Fatalf("catalog family/effort variant caused a model rewrite: note=%q input=%s", note, out.UpdatedInput)
	}
}

func TestHandle_FamilyWithoutVariantFallsBackToTierDefault(t *testing.T) {
	decidertest.Use(t, decidertest.Tier(core.TierSmall))
	// Grok has no effort-tagged variants: a confidently trivial task falls
	// back to the tier-default small model, exactly like the legacy path.
	ev := cursor.Event{
		ToolName: "Task",
		ToolInput: json.RawMessage(`{
			"task": "fix a typo in the readme",
			"model": "grok-4.7-high-fast"
		}`),
	}
	out, _, _ := cursor.Handle(withCatalogSession(ev), cat)
	m := decodeUpdated(t, out)
	wantID := catID(core.TierSmall)
	if m["model"] != wantID {
		t.Errorf("model = %v, want tier default %s", m["model"], wantID)
	}
}

func TestHandle_UnknownIDIsNotRewritten(t *testing.T) {
	// An id this harness does not own stays put. Replacing it with a
	// catalog id from another namespace is what broke Cursor Task calls.
	ev := cursor.Event{
		ToolName: "Task",
		ToolInput: json.RawMessage(`{
			"task": "rename the userId variable to userIdentifier",
			"model": "some-future-model-9"
		}`),
	}
	out, note, _ := cursor.Handle(withCatalogSession(ev), cat)
	if note != "" || out.UpdatedInput != nil {
		t.Fatalf("foreign id must not be rewritten, note=%q updated=%s", note, out.UpdatedInput)
	}
}

func TestHandle_CursorUpshiftDoesNotEmitRejectedMuseSpark(t *testing.T) {
	decidertest.Use(t, decidertest.Tier(core.TierFrontier))
	const rejected = "muse-spark-1.3-high"
	muse, ok := cat.LookupByID("cursor", "muse-spark")
	if !ok {
		t.Fatal("catalog missing muse-spark alias")
	}
	if muse.ID == rejected || muse.WriteName() == rejected {
		t.Fatalf("catalog WriteName = %q, Cursor Task rejects that slug", muse.WriteName())
	}
	if other, found := cat.LookupByID("claude-code", muse.ID); found && other.ID == muse.ID && other.Harness == "claude-code" {
		t.Fatalf("muse catalog id %s must stay on cursor", muse.ID)
	}

	now := time.Now()
	window := func(used float64, models []string) *quota.Snapshot {
		scope := "models"
		if len(models) == 0 {
			scope = "harness"
		}
		return &quota.Snapshot{
			Version: 1, Harness: "cursor", Source: "cursor-usage",
			ObservedAt: now.Add(-time.Minute), ExpiresAt: now.Add(time.Minute),
			Windows: []quota.Window{{
				ID: "pool", Scope: scope, ModelIDs: models,
				UsedPercent: &used, ResetsAt: now.Add(time.Hour),
			}},
		}
	}
	includedCatalog := []string{muse.ID}
	unavailableSlug := []string{rejected}

	tests := []struct {
		name        string
		included    *[]string
		unavailable *[]string
		usage       *quota.Snapshot
		missingFile bool
		wantEmit    bool
		wantHeld    bool
		wantUnknown bool
	}{
		{name: "unavailable", included: &includedCatalog, unavailable: &unavailableSlug, wantHeld: true},
		{name: "exhausted", included: &includedCatalog, usage: window(100, []string{rejected}), wantHeld: true},
		{name: "unknown", missingFile: true, wantUnknown: true},
		{name: "available", included: &includedCatalog, wantEmit: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.missingFile {
				t.Setenv("DOWNSHIFT_CURSOR_NATIVE_FILE", filepath.Join(t.TempDir(), "no-native.json"))
				t.Setenv("DOWNSHIFT_QUOTA_FILE", filepath.Join(t.TempDir(), "no-quota.json"))
			} else {
				// A missing native export is treated as present and would close
				// QuotaAllows. Clear it so only this call's marks are evidence.
				t.Setenv("DOWNSHIFT_CURSOR_NATIVE_FILE", "")
				t.Setenv("DOWNSHIFT_QUOTA_FILE", filepath.Join(t.TempDir(), "no-quota.json"))
			}
			session := []string{"composer-2.5-fast", "composer-2.5", rejected}
			ev := cursor.Event{
				ToolName:          "Task",
				SessionModels:     &session,
				IncludedModels:    tc.included,
				UnavailableModels: tc.unavailable,
				UsageQuota:        tc.usage,
				ToolInput: json.RawMessage(`{
					"task": "rearchitect the payment flow across services",
					"model": "composer-2.5"
				}`),
			}
			out, note, decision := cursor.Handle(ev, cat)
			raw, err := json.Marshal(out)
			if err != nil {
				t.Fatalf("marshal output: %v", err)
			}
			blob := string(raw) + note
			if strings.Contains(blob, rejected) {
				t.Fatalf("emitted %s: %s", rejected, blob)
			}
			if !tc.wantEmit && strings.Contains(blob, "muse-spark") {
				t.Fatalf("emitted muse-spark without pool evidence: %s", blob)
			}
			written := ""
			if m := decodeUpdated(t, out); m != nil {
				written, _ = m["model"].(string)
			}
			if tc.wantEmit {
				if written != muse.ID || note == "" {
					t.Fatalf("model = %q note = %q, want catalog id %q and a rewrite note", written, note, muse.ID)
				}
				if decision.Model.ID != muse.ID {
					t.Fatalf("decision model = %q, want catalog id %q", decision.Model.ID, muse.ID)
				}
				if decision.Session.CreditHeld {
					t.Fatal("available pool must not set CreditHeld")
				}
				return
			}
			if out.UpdatedInput != nil || note != "" {
				t.Fatalf("spawn rewritten, note=%q updated=%s", note, out.UpdatedInput)
			}
			if decision.Session.CreditHeld != tc.wantHeld {
				t.Fatalf("CreditHeld = %v, want %v", decision.Session.CreditHeld, tc.wantHeld)
			}
			if tc.wantUnknown && (decision.Session.QuotaStatus != string(quota.Unknown) || decision.Session.CreditHeld) {
				t.Fatalf("unobserved pool status=%q creditHeld=%v, want unknown and not held", decision.Session.QuotaStatus, decision.Session.CreditHeld)
			}
		})
	}
}

func TestHandle_UnlabeledSessionIDIsEligible(t *testing.T) {
	decidertest.Use(t, decidertest.Tier(core.TierSmall))
	frontierID := catID(core.TierFrontier)
	session := []string{"composer-2.5"}
	ev := cursor.Event{
		ToolName:      "Task",
		SessionModels: &session,
		ToolInput: json.RawMessage(`{
			"task": "rename the userId variable to userIdentifier",
			"model": "` + frontierID + `"
		}`),
	}
	out, _, _ := cursor.Handle(ev, cat)
	m := decodeUpdated(t, out)
	if m == nil || m["model"] != "composer-2.5" {
		t.Fatalf("model = %v, want composer-2.5", m)
	}
}

func TestMain(m *testing.M) {
	os.Setenv("DOWNSHIFT_STATE_DIR", filepath.Join(os.TempDir(), "downshift-test-state-cursor"))
	os.Setenv("DOWNSHIFT_SESSION_MODELS", filepath.Join(os.TempDir(), "downshift-session-models-absent.json"))
	os.Setenv("DOWNSHIFT_DISCOVERY", "off")
	os.Exit(m.Run())
}

// --- Held decisions must not rewrite via the family path ---

// foreignFamilyResolver recommends a foreign-harness model (R4 hold) while
// still offering an in-session same-family variant, isolating the family gate.
type foreignFamilyResolver struct {
	core.Resolver
	target core.Model
	family core.Model
}

func (s foreignFamilyResolver) ModelFor(harness string, tier core.Tier) core.Model {
	return s.target
}

func (s foreignFamilyResolver) FamilyModelFor(harness, family string, effort core.Effort) (core.Model, bool) {
	if s.family.ID == "" {
		return core.Model{}, false
	}
	return s.family, true
}

func TestHandle_HeldDecisionDoesNotRewriteViaFamilyPath(t *testing.T) {
	// Current opus high, COMPLEX task: tiers match (OK gear), but the
	// recommended model is foreign-harness, so the decision is held (R4)
	// and the session plan says "do not rewrite". Before the hold gate,
	// the in-family medium variant rewrote anyway.
	familyTarget, ok := cat.LookupByID("cursor", "claude-opus-5.5-medium")
	if !ok {
		t.Fatal("catalog missing claude-opus-5.5-medium")
	}
	res := foreignFamilyResolver{
		Resolver: cat,
		target:   core.Model{ID: "foreign-opus-9", Harness: "codex", Tier: core.TierFrontier},
		family:   familyTarget,
	}
	ev := cursor.Event{
		ToolName: "Task",
		ToolInput: json.RawMessage(`{
			"task": "rearchitect the payment flow across services with a data migration",
			"model": "claude-opus-5-thinking-high"
		}`),
	}
	out, note, decision := cursor.Handle(withCatalogSession(ev), res)
	if len(decision.Corrections) == 0 {
		t.Fatal("expected a held decision (R4), got none — test is vacuous")
	}
	if out.UpdatedInput != nil {
		m := decodeUpdated(t, out)
		t.Errorf("held decision must not rewrite via family path, got model=%v", m["model"])
	}
	if note != "" {
		t.Errorf("expected no note when a held decision falls through to allow, got %q", note)
	}
}

func TestProjectDirFallsBackToWorkspaceRoots(t *testing.T) {
	cases := []struct {
		name string
		ev   cursor.Event
		want string
	}{
		{"cwd wins", cursor.Event{Cwd: "/a", WorkspaceRoots: []string{"/b"}}, "/a"},
		{"empty cwd uses first root", cursor.Event{Cwd: "", WorkspaceRoots: []string{"/b", "/c"}}, "/b"},
		{"none", cursor.Event{}, ""},
	}
	for _, c := range cases {
		if got := c.ev.ProjectDir(); got != c.want {
			t.Errorf("%s: ProjectDir() = %q, want %q", c.name, got, c.want)
		}
	}
	var ev cursor.Event
	if err := json.Unmarshal([]byte(`{"workspace_roots":["/w"]}`), &ev); err != nil || ev.ProjectDir() != "/w" {
		t.Errorf("workspace_roots must parse from the payload, got %q (%v)", ev.ProjectDir(), err)
	}
}
