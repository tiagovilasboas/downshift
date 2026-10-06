// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package codex_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/adapters/codex"
	"github.com/tiagovilasboas/downshift/internal/catalog"
	"github.com/tiagovilasboas/downshift/internal/core"
)

var cat = catalog.Load()

func TestEventTaskText(t *testing.T) {
	ev := codex.Event{SessionID: "session-123", ToolInput: json.RawMessage(`{"message":"private task","task_name":"security review"}`)}
	if got := ev.TaskText(); got != "private task security review" {
		t.Fatalf("TaskText() = %q", got)
	}
	if got := ev.SessionIdentifier(); got != "session-123" {
		t.Fatalf("SessionIdentifier() = %q", got)
	}
}

func TestHandle_UsesSessionScopedAllowlist(t *testing.T) {
	t.Setenv("DOWNSHIFT_SESSION_MODELS", filepath.Join(t.TempDir(), "session-models.json"))
	path := os.Getenv("DOWNSHIFT_SESSION_MODELS")
	data := []byte(`{"codex":["gpt-5.6-sol"],"sessions":{"codex":{"session-123":["gpt-6-luna","gpt-6-sol"]}}}`)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	ev := codex.Event{
		SessionID: "session-123",
		ToolName:  "spawn_agent",
		Model:     "gpt-6-sol",
		ToolInput: json.RawMessage(`{"message":"rename a local variable"}`),
	}
	out, _, _ := codex.Handle(ev, cat)
	m := decodeUpdated(t, out)
	if m["model"] != "gpt-6-luna" {
		t.Fatalf("session-scoped model = %v, want gpt-6-luna", m["model"])
	}
}

func catID(tier core.Tier) string {
	return cat.ModelFor("codex", tier).ID
}

func withCatalogSession(ev codex.Event) codex.Event {
	ids := []string{cat.ModelFor("codex", core.TierSmall).ID, cat.ModelFor("codex", core.TierMid).ID, cat.ModelFor("codex", core.TierFrontier).ID}
	ev.SessionModels = &ids
	return ev
}

func decodeUpdated(t *testing.T, out codex.Output) map[string]any {
	t.Helper()
	if out.HookSpecificOutput == nil || out.HookSpecificOutput.UpdatedInput == nil {
		return nil
	}
	var m map[string]any
	if err := json.Unmarshal(out.HookSpecificOutput.UpdatedInput, &m); err != nil {
		t.Fatalf("updatedInput not valid JSON: %v", err)
	}
	return m
}

func TestHandle_DownshiftsTrivialSubagent(t *testing.T) {
	frontierID := catID(core.TierFrontier)
	ev := codex.Event{
		ToolName: "spawn_agent",
		Model:    frontierID,
		ToolInput: json.RawMessage(`{
			"task_name": "worker_agent_rename",
			"message": "rename the userId variable to userIdentifier",
			"fork_turns": "none"
		}`),
	}
	out, note, _ := codex.Handle(withCatalogSession(ev), cat)
	if note == "" {
		t.Fatal("expected a downshift note, got none")
	}
	if out.HookSpecificOutput.PermissionDecision != "allow" {
		t.Errorf("permissionDecision = %s, want allow", out.HookSpecificOutput.PermissionDecision)
	}
	m := decodeUpdated(t, out)
	wantID := catID(core.TierSmall)
	if m["model"] != wantID {
		t.Errorf("model = %v, want %s", m["model"], wantID)
	}
	// reasoning_effort must be set (not empty).
	if m["reasoning_effort"] == "" || m["reasoning_effort"] == nil {
		t.Error("reasoning_effort must be set on downshift")
	}
	// Reserved schema fields must be preserved.
	if m["message"] == nil || m["task_name"] == nil || m["fork_turns"] == nil {
		t.Error("reserved spawn_agent fields must be preserved in updatedInput")
	}
}

func TestHandle_UpshiftsComplexSubagent(t *testing.T) {
	smallID := catID(core.TierSmall)
	ev := codex.Event{
		ToolName: "spawn_agent",
		Model:    smallID,
		ToolInput: json.RawMessage(`{
			"message": "rearchitect the payment flow across multiple services and migrate the schema"
		}`),
	}
	out, note, _ := codex.Handle(withCatalogSession(ev), cat)
	if note == "" {
		t.Fatal("expected an upshift note, got none")
	}
	m := decodeUpdated(t, out)
	wantID := catID(core.TierFrontier)
	if m["model"] != wantID {
		t.Errorf("model = %v, want %s", m["model"], wantID)
	}
}

func TestHandle_OKRewritesReasoningEffort(t *testing.T) {
	midID := catID(core.TierMid)
	ev := codex.Event{
		ToolName: "spawn_agent",
		Model:    midID,
		ToolInput: json.RawMessage(`{
			"message": "implement the CSV export feature",
			"model": "` + midID + `"
		}`),
	}
	out, note, _ := codex.Handle(withCatalogSession(ev), cat)
	if note == "" {
		t.Error("expected an effort-routing note")
	}
	m := decodeUpdated(t, out)
	if m["model"] != midID {
		t.Errorf("model = %v, want %s", m["model"], midID)
	}
	if m["reasoning_effort"] != "medium" {
		t.Errorf("reasoning_effort = %v, want medium", m["reasoning_effort"])
	}
}

func TestHandle_IgnoresNonSpawnTools(t *testing.T) {
	ev := codex.Event{
		ToolName:  "Bash",
		Model:     catID(core.TierFrontier),
		ToolInput: json.RawMessage(`{"command":"ls"}`),
	}
	_, note, _ := codex.Handle(withCatalogSession(ev), cat)
	if note != "" {
		t.Errorf("expected no note for non-spawn tool, got %q", note)
	}
}

func TestHandle_MatchesFlattenedNamespacedToolName(t *testing.T) {
	frontierID := catID(core.TierFrontier)
	ev := codex.Event{
		ToolName:  "collaborationspawn_agent",
		Model:     frontierID,
		ToolInput: json.RawMessage(`{"message": "fix a typo in the readme"}`),
	}
	out, note, _ := codex.Handle(withCatalogSession(ev), cat)
	if note == "" {
		t.Fatal("expected downshift for flattened namespaced tool name")
	}
	m := decodeUpdated(t, out)
	wantID := catID(core.TierSmall)
	if m["model"] != wantID {
		t.Errorf("model = %v, want %s", m["model"], wantID)
	}
}

func TestHandle_MatchesAgentToolName(t *testing.T) {
	frontierID := catID(core.TierFrontier)
	ev := codex.Event{
		ToolName:  "Agent",
		Model:     frontierID,
		ToolInput: json.RawMessage(`{"message": "rename a private helper method"}`),
	}
	_, note, _ := codex.Handle(withCatalogSession(ev), cat)
	if note == "" {
		t.Fatal("expected downshift for Agent tool name")
	}
}

func TestHandle_TaskNameAddsSignal(t *testing.T) {
	frontierID := catID(core.TierFrontier)
	ev := codex.Event{
		ToolName: "spawn_agent",
		Model:    frontierID,
		ToolInput: json.RawMessage(`{
			"task_name": "review_agent",
			"message": "typo fix",
			"reasoning_effort": "low"
		}`),
	}
	out, note, _ := codex.Handle(withCatalogSession(ev), cat)
	if note == "" {
		t.Fatal("expected a routing decision")
	}
	m := decodeUpdated(t, out)
	if m["task_name"] != "review_agent" {
		t.Errorf("task_name must be preserved, got %v", m["task_name"])
	}
}

func TestHandle_FallsBackToEventModel(t *testing.T) {
	frontierID := catID(core.TierFrontier)
	ev := codex.Event{
		ToolName:  "spawn_agent",
		Model:     frontierID,
		ToolInput: json.RawMessage(`{"message": "rename the variable"}`),
	}
	out, note, _ := codex.Handle(withCatalogSession(ev), cat)
	if note == "" {
		t.Fatal("expected downshift using event model as current")
	}
	m := decodeUpdated(t, out)
	wantID := catID(core.TierSmall)
	if m["model"] != wantID {
		t.Errorf("model = %v, want %s", m["model"], wantID)
	}
}

func TestHandle_UnknownCurrentModelStillRoutes(t *testing.T) {
	ev := codex.Event{ToolName: "spawn_agent", ToolInput: json.RawMessage(`{"message":"do a code review"}`)}
	out, note, _ := codex.Handle(withCatalogSession(ev), cat)
	if note == "" {
		t.Fatal("expected a routing note")
	}
	m := decodeUpdated(t, out)
	if m["model"] != catID(core.TierFrontier) {
		t.Errorf("model = %v, want %s", m["model"], catID(core.TierFrontier))
	}
}

func TestHandle_MalformedInputFailsOpen(t *testing.T) {
	ev := codex.Event{
		ToolName:  "spawn_agent",
		Model:     catID(core.TierFrontier),
		ToolInput: json.RawMessage(`{not valid json`),
	}
	out, note, _ := codex.Handle(withCatalogSession(ev), cat)
	if note != "" {
		t.Errorf("expected fail-open (no note), got %q", note)
	}
	if out.HookSpecificOutput.PermissionDecision != "allow" {
		t.Error("malformed input must fail open with permissionDecision: allow")
	}
}

func TestHandle_UsesSupportedCodexPreToolUseEnvelope(t *testing.T) {
	ev := codex.Event{
		ToolName:  "spawn_agent",
		Model:     catID(core.TierFrontier),
		ToolInput: json.RawMessage(`{"message": "rename the variable"}`),
	}
	out, _, _ := codex.Handle(withCatalogSession(ev), cat)
	raw, err := json.Marshal(out)
	if err != nil {
		t.Fatalf("marshal output: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("decode output: %v", err)
	}
	if _, found := decoded["continue"]; found {
		t.Error("PreToolUse output must not contain unsupported continue")
	}
}

func TestHandle_EmptyMessageFailsOpen(t *testing.T) {
	ev := codex.Event{
		ToolName:  "spawn_agent",
		Model:     catID(core.TierFrontier),
		ToolInput: json.RawMessage(`{"fork_turns":"none"}`),
	}
	out, note, _ := codex.Handle(withCatalogSession(ev), cat)
	if note != "" {
		t.Errorf("expected no decision for empty message, got %q", note)
	}
	if out.HookSpecificOutput.UpdatedInput != nil {
		t.Error("must not rewrite when there is no task text")
	}
}

// TestHandle_SiblingFieldsPreserved is a regression test ensuring that a Codex
// PreToolUse rewrite does not drop fields from the spawn_agent payload (the
// v2 schema has reserved fields that must all survive the model/effort rewrite).
func TestHandle_SiblingFieldsPreserved(t *testing.T) {
	frontierID := catID(core.TierFrontier)
	ev := codex.Event{
		ToolName: "spawn_agent",
		Model:    frontierID,
		ToolInput: json.RawMessage(`{
			"message":          "rename the userId variable",
			"task_name":        "worker_agent_rename",
			"fork_turns":       "none",
			"model":            "` + frontierID + `",
			"timeout_seconds":  120,
			"background":       false
		}`),
	}
	out, _, _ := codex.Handle(withCatalogSession(ev), cat)
	m := decodeUpdated(t, out)
	if m == nil {
		t.Fatal("expected updatedInput")
	}
	wantID := catID(core.TierSmall)
	if m["model"] != wantID {
		t.Errorf("model = %v, want %s", m["model"], wantID)
	}
	if m["reasoning_effort"] == nil {
		t.Error("reasoning_effort must be set")
	}
	// Reserved v2 fields must survive.
	for _, field := range []string{"message", "task_name", "fork_turns", "timeout_seconds", "background"} {
		if m[field] == nil {
			t.Errorf("field %q was dropped from updatedInput", field)
		}
	}
}

func TestMain(m *testing.M) {
	os.Setenv("DOWNSHIFT_SESSION_MODELS", filepath.Join(os.TempDir(), "downshift-session-models-absent.json"))
	os.Exit(m.Run())
}

func TestHandle_MissingSessionDoesNotRewrite(t *testing.T) {
	frontierID := catID(core.TierFrontier)
	ev := codex.Event{
		ToolName: "spawn_agent",
		ToolInput: json.RawMessage(`{
			"message": "rename the userId variable to userIdentifier",
			"model": "` + frontierID + `"
		}`),
	}
	out, note, _ := codex.Handle(ev, cat)
	if note != "" || (out.HookSpecificOutput != nil && out.HookSpecificOutput.UpdatedInput != nil) {
		t.Fatalf("missing session must not rewrite, note=%q", note)
	}
}

func TestHandle_SessionWithoutCatalogSmallUsesNextInSession(t *testing.T) {
	frontierID := catID(core.TierFrontier)
	session := []string{"gpt-5.6-terra", frontierID}
	ev := codex.Event{
		ToolName:      "spawn_agent",
		SessionModels: &session,
		ToolInput: json.RawMessage(`{
			"message": "rename the userId variable to userIdentifier",
			"model": "` + frontierID + `"
		}`),
	}
	out, _, _ := codex.Handle(ev, cat)
	m := decodeUpdated(t, out)
	if m == nil || m["model"] != "gpt-5.6-terra" {
		t.Fatalf("model = %v, want gpt-5.6-terra", m)
	}
}

// A spawn without a model and an unconfident small-tier classification must
// leave both the model and the effort untouched (guardrail R6).
func TestHandle_NoModelUnconfidentSmallDoesNotRewrite(t *testing.T) {
	session := []string{"gpt-5.6-luna", "gpt-5.6-terra", "gpt-5.6-sol"}
	ev := codex.Event{
		ToolName:      "spawn_agent",
		SessionModels: &session,
		ToolInput:     json.RawMessage(`{"message": "write a function to parse dates"}`),
	}
	out, note, d := codex.Handle(ev, cat)
	if d.Confident || d.Tier != core.TierSmall {
		t.Fatalf("precondition: want unconfident small, got tier=%v confident=%v", d.Tier, d.Confident)
	}
	if m := decodeUpdated(t, out); m != nil || note != "" {
		t.Fatalf("unconfident small without a model must not rewrite, got %v note=%q", m, note)
	}
}

// A downshift held by guardrail R1 must not lower the requested effort on
// the kept model, and the decision must record the model actually kept.
func TestHandle_HeldDownshiftKeepsRequestedEffort(t *testing.T) {
	frontierID := catID(core.TierFrontier)
	ev := withCatalogSession(codex.Event{
		ToolName:  "spawn_agent",
		ToolInput: json.RawMessage(`{"message": "look at the logs folder and tell me what you see", "model": "` + frontierID + `", "reasoning_effort": "high"}`),
	})
	out, note, d := codex.Handle(ev, cat)
	if len(d.Corrections) == 0 {
		t.Fatalf("precondition: want a held decision, got %+v", d)
	}
	if m := decodeUpdated(t, out); m != nil {
		t.Fatalf("held downshift must leave the spawn unchanged, got %v (note %q)", m, note)
	}
}

// An unconfident same-tier decision must not lower an explicit effort.
func TestHandle_UnconfidentOKDoesNotLowerEffort(t *testing.T) {
	midID := catID(core.TierMid)
	ev := withCatalogSession(codex.Event{
		ToolName:  "spawn_agent",
		ToolInput: json.RawMessage(`{"message": "look into why the build is slow", "model": "` + midID + `", "reasoning_effort": "xhigh"}`),
	})
	out, _, d := codex.Handle(ev, cat)
	if d.Confident || d.Verdict != core.VerdictOK {
		t.Fatalf("precondition: want unconfident OK, got verdict=%v confident=%v", d.Verdict, d.Confident)
	}
	if m := decodeUpdated(t, out); m != nil && m["reasoning_effort"] != "xhigh" {
		t.Fatalf("effort lowered on doubt: %v", m["reasoning_effort"])
	}
}

// A confident decision may still set effort (unchanged behaviour).
func TestHandle_ConfidentDownshiftSetsLowEffort(t *testing.T) {
	frontierID := catID(core.TierFrontier)
	ev := withCatalogSession(codex.Event{
		ToolName:  "spawn_agent",
		ToolInput: json.RawMessage(`{"message": "rename the userId variable to userIdentifier", "model": "` + frontierID + `", "reasoning_effort": "high"}`),
	})
	out, _, _ := codex.Handle(ev, cat)
	m := decodeUpdated(t, out)
	if m == nil || m["reasoning_effort"] == "high" {
		t.Fatalf("confident downshift should lower effort, got %v", m)
	}
}
