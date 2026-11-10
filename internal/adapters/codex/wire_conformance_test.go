// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package codex_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"github.com/tiagovilasboas/downshift-ai/internal/adapters/codex"
	"github.com/tiagovilasboas/downshift-ai/internal/core"
)

// codexPreToolUse applies the rules Codex uses to accept a PreToolUse hook
// answer on stdout and returns the updatedInput it would apply (nil for none).
// An error is an answer Codex marks as a failed hook run: it then runs the
// original tool call unchanged. Source: the Codex hooks engine
// (codex-rs/hooks/src/engine/output_parser.rs parse_pre_tool_use and
// unsupported_pre_tool_use_*, codex-rs/hooks/src/schema.rs
// PreToolUseCommandOutputWire, read at openai/codex 568c592f):
//   - unknown fields are rejected at the top level and in hookSpecificOutput;
//   - updatedInput is applied only with permissionDecision "allow";
//   - "allow" without updatedInput is unsupported, and so is "ask";
//   - "deny" needs a non-empty permissionDecisionReason;
//   - continue:false, stopReason and suppressOutput are unsupported.
func codexPreToolUse(stdout []byte) (json.RawMessage, error) {
	if len(bytes.TrimSpace(stdout)) == 0 {
		return nil, nil
	}
	type specific struct {
		HookEventName            string          `json:"hookEventName"`
		PermissionDecision       *string         `json:"permissionDecision"`
		PermissionDecisionReason *string         `json:"permissionDecisionReason"`
		UpdatedInput             json.RawMessage `json:"updatedInput"`
		AdditionalContext        *string         `json:"additionalContext"`
	}
	var wire struct {
		Continue           *bool     `json:"continue"`
		StopReason         *string   `json:"stopReason"`
		SuppressOutput     bool      `json:"suppressOutput"`
		SystemMessage      *string   `json:"systemMessage"`
		Decision           *string   `json:"decision"`
		Reason             *string   `json:"reason"`
		HookSpecificOutput *specific `json:"hookSpecificOutput"`
	}
	dec := json.NewDecoder(bytes.NewReader(stdout))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&wire); err != nil {
		return nil, fmt.Errorf("invalid pre-tool-use JSON output: %w", err)
	}
	switch {
	case wire.Continue != nil && !*wire.Continue:
		return nil, errors.New("unsupported continue:false")
	case wire.StopReason != nil:
		return nil, errors.New("unsupported stopReason")
	case wire.SuppressOutput:
		return nil, errors.New("unsupported suppressOutput")
	}
	h := wire.HookSpecificOutput
	if h == nil {
		return nil, nil
	}
	if h.HookEventName != "PreToolUse" {
		return nil, fmt.Errorf("hookEventName %q", h.HookEventName)
	}
	decision := ""
	if h.PermissionDecision != nil {
		decision = *h.PermissionDecision
	}
	hasUpdate := len(h.UpdatedInput) > 0 && string(h.UpdatedInput) != "null"
	switch {
	case hasUpdate && decision != "allow":
		return nil, errors.New("updatedInput without permissionDecision:allow")
	case decision == "allow" && !hasUpdate:
		return nil, errors.New("unsupported permissionDecision:allow")
	case decision == "ask":
		return nil, errors.New("unsupported permissionDecision:ask")
	case decision == "deny" && (h.PermissionDecisionReason == nil || *h.PermissionDecisionReason == ""):
		return nil, errors.New("deny without a reason")
	case decision != "" && decision != "allow" && decision != "deny":
		return nil, fmt.Errorf("unknown permissionDecision %q", decision)
	}
	if !hasUpdate {
		return nil, nil
	}
	return h.UpdatedInput, nil
}

// The validator itself must reject the shapes Codex rejects; otherwise the
// conformance tests below prove nothing.
func TestCodexPreToolUseValidatorRejects(t *testing.T) {
	for name, stdout := range map[string]string{
		"rewrite without allow": `{"hookSpecificOutput":{"hookEventName":"PreToolUse","updatedInput":{"command":"x"}}}`,
		"bare allow":            `{"hookSpecificOutput":{"hookEventName":"PreToolUse","permissionDecision":"allow"}}`,
		"ask":                   `{"hookSpecificOutput":{"hookEventName":"PreToolUse","permissionDecision":"ask","updatedInput":{"command":"x"}}}`,
		"unknown field":         `{"hookSpecificOutput":{"hookEventName":"PreToolUse","permissionDecision":"allow","updatedInput":{},"extra":1}}`,
		"continue false":        `{"continue":false}`,
	} {
		if _, err := codexPreToolUse([]byte(stdout)); err == nil {
			t.Errorf("%s: accepted %s", name, stdout)
		}
	}
}

// The shell rewrite is the answer that swaps a Bash command for its
// `downshift compact` form. Codex must accept it and apply updatedInput with
// every sibling field of tool_input intact.
func TestShellRewrite_ConformsToCodexWire(t *testing.T) {
	ev := codex.Event{ToolName: "Bash", ToolInput: json.RawMessage(`{"command":"git status","workdir":"/w","timeout_ms":5000}`)}
	out, err := ev.ShellRewrite("downshift compact -- git status")
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := json.Marshal(out)
	if err != nil {
		t.Fatal(err)
	}
	updated, err := codexPreToolUse(stdout)
	if err != nil {
		t.Fatalf("Codex would fail this hook and run the original command: %v\n%s", err, stdout)
	}
	var ti map[string]any
	if err := json.Unmarshal(updated, &ti); err != nil {
		t.Fatal(err)
	}
	if ti["command"] != "downshift compact -- git status" || ti["workdir"] != "/w" || ti["timeout_ms"] != float64(5000) || len(ti) != 3 {
		t.Fatalf("updatedInput = %v", ti)
	}
}

// The spawn routing answers: a rewrite (model or effort) and the neutral
// no-opinion answer. Both must pass Codex's validation; only the rewrite
// carries updatedInput.
func TestHandle_ConformsToCodexWire(t *testing.T) {
	rewrite := codex.Event{
		ToolName:  "spawn_agent",
		Model:     catID(core.TierMid),
		ToolInput: json.RawMessage(`{"message":"implement the CSV export feature","model":"` + catID(core.TierMid) + `"}`),
	}
	neutral := codex.Event{
		ToolName:  "spawn_agent",
		Model:     catID(core.TierFrontier),
		ToolInput: json.RawMessage(`{"fork_turns":"none"}`),
	}
	for name, tc := range map[string]struct {
		ev         codex.Event
		wantUpdate bool
	}{
		"rewrite": {rewrite, true},
		"neutral": {neutral, false},
	} {
		out, _, _ := codex.Handle(withCatalogSession(tc.ev), cat)
		stdout, err := json.Marshal(out)
		if err != nil {
			t.Fatal(err)
		}
		updated, err := codexPreToolUse(stdout)
		if err != nil {
			t.Errorf("%s: Codex would fail this hook: %v\n%s", name, err, stdout)
			continue
		}
		if (updated != nil) != tc.wantUpdate {
			t.Errorf("%s: updatedInput applied = %v, want %v (%s)", name, updated != nil, tc.wantUpdate, stdout)
		}
	}
}
