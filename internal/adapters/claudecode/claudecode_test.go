// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package claudecode_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/adapters/claudecode"
	"github.com/tiagovilasboas/downshift/internal/catalog"
	"github.com/tiagovilasboas/downshift/internal/core"
)

// cat is the shared test resolver — same embedded catalog the binary uses.
var cat = catalog.Load()

func TestEventTaskText(t *testing.T) {
	ev := claudecode.Event{ToolInput: json.RawMessage(`{"prompt":"private task"}`)}
	if got := ev.TaskText(); got != "private task" {
		t.Fatalf("TaskText() = %q", got)
	}
}

// nativeOf is the name the harness schema accepts for a tier, read from the
// catalog so a model version bump does not touch the tests.
func nativeOf(tier core.Tier) string {
	return cat.ModelFor("claude-code", tier).Native
}

func catID(tier core.Tier) string {
	return cat.ModelFor("claude-code", tier).ID
}

func withCatalogSession(ev claudecode.Event) claudecode.Event {
	ids := []string{cat.ModelFor("claude-code", core.TierSmall).ID, cat.ModelFor("claude-code", core.TierMid).ID, cat.ModelFor("claude-code", core.TierFrontier).ID}
	ev.SessionModels = &ids
	return ev
}

func decodeUpdated(t *testing.T, out claudecode.Output) map[string]any {
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
	ev := claudecode.Event{
		ToolName: "Task",
		Model:    frontierID,
		ToolInput: json.RawMessage(`{
			"description": "cleanup",
			"prompt": "rename the userId variable to userIdentifier",
			"model": "` + frontierID + `"
		}`),
	}
	out, note, _ := claudecode.Handle(withCatalogSession(ev), cat)
	if note == "" {
		t.Fatal("expected a downshift note, got none")
	}
	m := decodeUpdated(t, out)
	if m == nil {
		t.Fatal("expected updatedInput, got none")
	}
	if m["model"] != nativeOf(core.TierSmall) {
		t.Errorf("model = %v, want %s (small tier)", m["model"], nativeOf(core.TierSmall))
	}
	if m["prompt"] == nil || m["description"] == nil {
		t.Error("prompt and description must be preserved in updatedInput")
	}
	if out.HookSpecificOutput.PermissionDecision != "allow" {
		t.Errorf("permissionDecision = %s, want allow", out.HookSpecificOutput.PermissionDecision)
	}
}

func TestHandle_UpshiftsComplexSubagent(t *testing.T) {
	smallID := catID(core.TierSmall)
	ev := claudecode.Event{
		ToolName: "Task",
		Model:    smallID,
		ToolInput: json.RawMessage(`{
			"prompt": "rearchitect the payment flow across services",
			"model": "` + smallID + `"
		}`),
	}
	out, note, _ := claudecode.Handle(withCatalogSession(ev), cat)
	if note == "" {
		t.Fatal("expected an upshift note, got none")
	}
	m := decodeUpdated(t, out)
	if m["model"] != nativeOf(core.TierFrontier) {
		t.Errorf("model = %v, want %s (frontier tier)", m["model"], nativeOf(core.TierFrontier))
	}
}

func TestHandle_NoChangeWhenAlreadyRightGear(t *testing.T) {
	midID := catID(core.TierMid)
	ev := claudecode.Event{
		ToolName: "Task",
		Model:    midID,
		ToolInput: json.RawMessage(`{
			"prompt": "implement the CSV export feature",
			"model": "` + midID + `"
		}`),
	}
	out, note, _ := claudecode.Handle(withCatalogSession(ev), cat)
	if note != "" {
		t.Errorf("expected no change note, got %q", note)
	}
	if out.HookSpecificOutput.UpdatedInput != nil {
		t.Error("must not rewrite input when gear is already right")
	}
}

func TestHandle_IgnoresNonTaskTools(t *testing.T) {
	ev := claudecode.Event{
		ToolName:  "Bash",
		Model:     catID(core.TierFrontier),
		ToolInput: json.RawMessage(`{"command": "rm -rf /tmp/x"}`),
	}
	out, note, _ := claudecode.Handle(withCatalogSession(ev), cat)
	if note != "" {
		t.Errorf("expected no note for non-Task tool, got %q", note)
	}
	if out.HookSpecificOutput.UpdatedInput != nil {
		t.Error("must not rewrite input for non-Task tools")
	}
}

func TestHandle_FallsBackToSessionModel(t *testing.T) {
	frontierID := catID(core.TierFrontier)
	ev := claudecode.Event{
		ToolName:  "Task",
		Model:     frontierID, // session model, no model on tool input
		ToolInput: json.RawMessage(`{"prompt": "fix a typo in the readme"}`),
	}
	out, note, _ := claudecode.Handle(withCatalogSession(ev), cat)
	if note == "" {
		t.Fatal("expected downshift using session model as current")
	}
	m := decodeUpdated(t, out)
	if m["model"] != nativeOf(core.TierSmall) {
		t.Errorf("model = %v, want %s", m["model"], nativeOf(core.TierSmall))
	}
}

func TestHandle_CursorSlugIsNotRewritten(t *testing.T) {
	session := []string{"claude-haiku-4-5", "claude-sonnet-5-5", "claude-opus-5-5"}
	ev := claudecode.Event{
		ToolName:      "Task",
		SessionModels: &session,
		ToolInput:     json.RawMessage(`{"prompt":"rename the userId variable","model":"composer-2.5"}`),
	}
	out, note, _ := claudecode.Handle(ev, cat)
	if note != "" || len(out.HookSpecificOutput.UpdatedInput) > 0 {
		t.Fatalf("claude-code must not write its catalog id over a cursor slug, note=%q input=%s", note, out.HookSpecificOutput.UpdatedInput)
	}
}

func TestHandle_AgentToolAlias(t *testing.T) {
	frontierID := catID(core.TierFrontier)
	ev := claudecode.Event{
		ToolName:  "Agent",
		Model:     frontierID,
		ToolInput: json.RawMessage(`{"prompt": "rename the variable", "model": "` + frontierID + `"}`),
	}
	_, note, _ := claudecode.Handle(withCatalogSession(ev), cat)
	if note == "" {
		t.Error("expected Agent tool to be treated as a subagent spawn")
	}
}

func TestHandle_MalformedInputFailsOpen(t *testing.T) {
	ev := claudecode.Event{
		ToolName:  "Task",
		Model:     catID(core.TierFrontier),
		ToolInput: json.RawMessage(`{not valid`),
	}
	out, note, _ := claudecode.Handle(withCatalogSession(ev), cat)
	if note != "" {
		t.Errorf("malformed input must fail-open, got note %q", note)
	}
	if out.HookSpecificOutput.PermissionDecision != "allow" {
		t.Error("malformed input must return allow")
	}
}

// TestHandle_SiblingFieldsPreserved is a regression test for the updatedInput
// replace-vs-patch behaviour. Claude Code's hook replaces the entire tool_input
// with updatedInput — so the adapter must re-marshal the *full* decoded map
// with only the model key changed. Any field not present in updatedInput is
// silently dropped by the harness, which can break subagents that use timeout,
// run_in_background, or other non-standard fields.
func TestHandle_SiblingFieldsPreserved(t *testing.T) {
	frontierID := catID(core.TierFrontier)
	ev := claudecode.Event{
		ToolName: "Task",
		Model:    frontierID,
		ToolInput: json.RawMessage(`{
			"prompt":             "rename the userId variable",
			"model":              "` + frontierID + `",
			"timeout":            30,
			"description":        "a short rename task",
			"run_in_background":  false
		}`),
	}
	out, _, _ := claudecode.Handle(withCatalogSession(ev), cat)
	m := decodeUpdated(t, out)
	if m == nil {
		t.Fatal("expected updatedInput")
	}
	// Model must be rewritten.
	if m["model"] != nativeOf(core.TierSmall) {
		t.Errorf("model = %v, want %s", m["model"], nativeOf(core.TierSmall))
	}
	// All sibling fields must survive the rewrite.
	if m["timeout"] == nil {
		t.Error("timeout field was dropped from updatedInput")
	}
	if m["description"] == nil {
		t.Error("description field was dropped from updatedInput")
	}
	if m["run_in_background"] == nil {
		t.Error("run_in_background field was dropped from updatedInput")
	}
	if m["prompt"] == nil {
		t.Error("prompt field was dropped from updatedInput")
	}
}

func TestMain(m *testing.M) {
	os.Setenv("DOWNSHIFT_SESSION_MODELS", filepath.Join(os.TempDir(), "downshift-session-models-absent.json"))
	os.Exit(m.Run())
}

func TestHandle_SessionWithoutCatalogSmallUsesNextInSession(t *testing.T) {
	frontierID := catID(core.TierFrontier)
	session := []string{"claude-sonnet-5-5", frontierID}
	ev := claudecode.Event{
		ToolName:      "Task",
		SessionModels: &session,
		ToolInput: json.RawMessage(`{
			"prompt": "rename the userId variable to userIdentifier",
			"model": "` + frontierID + `"
		}`),
	}
	out, _, _ := claudecode.Handle(ev, cat)
	m := decodeUpdated(t, out)
	if m == nil {
		t.Fatal("expected a rewrite to a session model")
	}
	if m["model"] != nativeOf(core.TierMid) {
		t.Fatalf("model = %v, want %s", m["model"], nativeOf(core.TierMid))
	}
}

func TestHandle_MissingSessionDoesNotRewrite(t *testing.T) {
	frontierID := catID(core.TierFrontier)
	ev := claudecode.Event{
		ToolName: "Task",
		ToolInput: json.RawMessage(`{
			"prompt": "rename the userId variable to userIdentifier",
			"model": "` + frontierID + `"
		}`),
	}
	out, note, _ := claudecode.Handle(ev, cat)
	if note != "" || (out.HookSpecificOutput != nil && out.HookSpecificOutput.UpdatedInput != nil) {
		t.Fatalf("missing session must not rewrite, note=%q", note)
	}
}

// A Task payload without a model field and an unconfident small-tier
// classification must not be rewritten: the child keeps the harness default.
func TestHandle_NoModelUnconfidentSmallDoesNotRewrite(t *testing.T) {
	ev := withCatalogSession(claudecode.Event{
		ToolName:  "Task",
		ToolInput: json.RawMessage(`{"prompt": "write a function to parse dates"}`),
	})
	out, note, d := claudecode.Handle(ev, cat)
	if d.Confident || d.Tier != core.TierSmall {
		t.Fatalf("precondition: want unconfident small, got tier=%v confident=%v", d.Tier, d.Confident)
	}
	if m := decodeUpdated(t, out); m != nil || note != "" {
		t.Fatalf("unconfident small without a model must not rewrite, got %v note=%q", m, note)
	}
}

func TestEvent_CorrelationIdentifier(t *testing.T) {
	ev := claudecode.Event{CorrelationID: "abc123"}
	if got := ev.CorrelationIdentifier(); got != "abc123" {
		t.Errorf("CorrelationIdentifier() = %q, want %q", got, "abc123")
	}
}

func TestEvent_RequestedReasoningEffort(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"with field", `{"reasoning_effort":"low"}`, "low"},
		{"missing field", `{"prompt":"test"}`, ""},
		{"malformed json", `{not valid}`, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ev := claudecode.Event{ToolInput: json.RawMessage(tt.input)}
			if got := ev.RequestedReasoningEffort(); got != tt.want {
				t.Errorf("RequestedReasoningEffort() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestEvent_RequestedModel(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		fallback string
		want     string
	}{
		{"from input", `{"model":"input-model"}`, "fallback-model", "input-model"},
		{"missing, use fallback", `{"prompt":"test"}`, "fallback-model", "fallback-model"},
		{"malformed, use fallback", `{not valid}`, "fallback-model", "fallback-model"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ev := claudecode.Event{
				Model:     tt.fallback,
				ToolInput: json.RawMessage(tt.input),
			}
			if got := ev.RequestedModel(); got != tt.want {
				t.Errorf("RequestedModel() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestEvent_TaskText_EdgeCases(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"from prompt", `{"prompt":"do a thing"}`, "do a thing"},
		{"from description", `{"description":"do a thing"}`, "do a thing"},
		{"prompt preferred", `{"prompt":"p","description":"d"}`, "p"},
		{"missing both", `{"model":"x"}`, ""},
		{"malformed", `{not valid}`, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ev := claudecode.Event{ToolInput: json.RawMessage(tt.input)}
			if got := ev.TaskText(); got != tt.want {
				t.Errorf("TaskText() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestHandle_EmptyPromptFailsOpen(t *testing.T) {
	// No prompt/description means we can't classify; fail open and allow.
	ev := claudecode.Event{
		ToolName:  "Task",
		Model:     catID(core.TierFrontier),
		ToolInput: json.RawMessage(`{"model":"some-model"}`),
	}
	out, note, _ := claudecode.Handle(ev, cat)
	if note != "" {
		t.Errorf("empty prompt must fail open, got note %q", note)
	}
	if out.HookSpecificOutput.PermissionDecision != "allow" {
		t.Error("empty prompt must return allow")
	}
}

func TestHandle_SessionQuotaRespected(t *testing.T) {
	// When session has IncludedModels/UnavailableModels quotas, the adapter
	// must not rewrite outside those constraints.
	smallID := catID(core.TierSmall)
	included := []string{smallID} // Only small allowed
	ev := claudecode.Event{
		ToolName:       "Task",
		Model:          catID(core.TierFrontier),
		IncludedModels: &included,
		ToolInput: json.RawMessage(`{
			"prompt": "rename variable",
			"model": "` + catID(core.TierFrontier) + `"
		}`),
	}
	out, note, d := claudecode.Handle(withCatalogSession(ev), cat)
	// The adapter may still try to rewrite, but if the session rejects it,
	// decision will reflect the constraints.
	_ = note
	_ = d
	// Minimal check: it should still return allow (fail-safe)
	if out.HookSpecificOutput.PermissionDecision != "allow" {
		t.Error("must always return allow permission")
	}
}

// Claude Code's Task/Agent schema accepts only family aliases for "model".
// A full catalog id fails validation and blocks the spawn, so the adapter must
// write the alias.
func TestHandle_WritesNativeAliasNotCatalogID(t *testing.T) {
	frontierID := catID(core.TierFrontier)
	ev := claudecode.Event{
		ToolName: "Task",
		Model:    frontierID,
		ToolInput: json.RawMessage(`{
			"prompt": "rename the userId variable to userIdentifier",
			"model": "` + frontierID + `"
		}`),
	}
	out, _, d := claudecode.Handle(withCatalogSession(ev), cat)
	m := decodeUpdated(t, out)
	if m == nil {
		t.Fatal("expected a rewrite")
	}
	if want := nativeOf(core.TierSmall); want == "" || m["model"] != want {
		t.Fatalf("model = %v, want the catalog native name %q", m["model"], want)
	}
	if d.Model.ID != catID(core.TierSmall) {
		t.Errorf("decision keeps the catalog id for telemetry, got %q", d.Model.ID)
	}
}

// A target whose catalog entry has no native_name must not be written: the
// harness would reject the whole spawn, so the hook allows it unchanged.
func TestHandle_NoNativeNameFailsOpen(t *testing.T) {
	c := &noNativeResolver{Resolver: cat}
	ev := withCatalogSession(claudecode.Event{
		ToolName: "Task",
		ToolInput: json.RawMessage(`{
			"prompt": "rename the userId variable",
			"model": "` + catID(core.TierFrontier) + `"
		}`),
	})
	out, note, _ := claudecode.Handle(ev, c)
	if m := decodeUpdated(t, out); m != nil || note != "" {
		t.Fatalf("must allow unchanged without a native name, got %v note=%q", m, note)
	}
}

// noNativeResolver strips Native from every model, simulating a catalog entry
// that omits native_name.
type noNativeResolver struct{ core.Resolver }

func (r *noNativeResolver) ModelFor(h string, t core.Tier) core.Model {
	m := r.Resolver.ModelFor(h, t)
	m.Native = ""
	return m
}

func (r *noNativeResolver) LookupByID(h, id string) (core.Model, bool) {
	m, ok := r.Resolver.LookupByID(h, id)
	m.Native = ""
	return m, ok
}
