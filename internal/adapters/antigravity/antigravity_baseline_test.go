// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package antigravity_test

import (
	"encoding/json"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/adapters/antigravity"
)

// Characterization: the requested model is the Model of the last subagent
// that has a Prompt, the same child whose decision Handle returns. "inherit"
// passes through unresolved; an absent Model gives an empty baseline.
func TestRequestedModel_Baseline(t *testing.T) {
	cases := []struct {
		name string
		args string
		want string
	}{
		{name: "single subagent", args: `{"Subagents":[{"Prompt":"p","Model":"pro"}]}`, want: "pro"},
		{name: "last subagent with prompt wins", args: `{"Subagents":[{"Prompt":"a","Model":"flash_lite"},{"Prompt":"b","Model":"pro"}]}`, want: "pro"},
		{name: "trailing subagent without prompt is skipped", args: `{"Subagents":[{"Prompt":"a","Model":"pro"},{"Model":"flash_lite"}]}`, want: "pro"},
		{name: "last with prompt but no Model resets to empty", args: `{"Subagents":[{"Prompt":"a","Model":"pro"},{"Prompt":"b"}]}`, want: ""},
		{name: "inherit passes through", args: `{"Subagents":[{"Prompt":"p","Model":"inherit"}]}`, want: "inherit"},
		{name: "absent Model", args: `{"Subagents":[{"Prompt":"p"}]}`, want: ""},
		{name: "no subagents", args: `{"Prompt":"p","Model":"pro"}`, want: ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ev := antigravity.Event{ToolCall: antigravity.ToolCall{Name: "invoke_subagent", Args: json.RawMessage(tc.args)}}
			if got := ev.RequestedModel(); got != tc.want {
				t.Fatalf("RequestedModel() = %q, want %q", got, tc.want)
			}
		})
	}
}

// With several subagents, one routing row describes one child: the last one that
// has a Prompt. The task text, the requested model and the decision must all be
// about that same child, or the row pairs one task with another child's tier.
func TestDecidedSubagentIsTheSameForTextModelAndDecision(t *testing.T) {
	const trivial = "fix typo in README"
	const heavy = "rearchitect the distributed consensus protocol and multi-region database replication"
	for _, tc := range []struct {
		name, args, wantText, wantModel string
	}{
		{"last child decides", `{"Subagents":[{"Prompt":"` + trivial + `","Model":"flash"},{"Prompt":"` + heavy + `","Model":"pro"}]}`, heavy, "pro"},
		{"last child with no Model: unknown, not the earlier one's", `{"Subagents":[{"Prompt":"` + trivial + `","Model":"flash"},{"Prompt":"` + heavy + `"}]}`, heavy, ""},
		{"a trailing child with no Prompt is skipped", `{"Subagents":[{"Prompt":"` + heavy + `","Model":"pro"},{"Model":"flash"}]}`, heavy, "pro"},
		{"one child", `{"Subagents":[{"Prompt":"` + trivial + `","Model":"flash"}]}`, trivial, "flash"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ev := antigravity.Event{ToolCall: antigravity.ToolCall{Name: "invoke_subagent", Args: json.RawMessage(tc.args)}}
			if got := ev.TaskText(); got != tc.wantText {
				t.Errorf("TaskText() = %q, want %q", got, tc.wantText)
			}
			if got := ev.RequestedModel(); got != tc.wantModel {
				t.Errorf("RequestedModel() = %q, want %q", got, tc.wantModel)
			}
		})
	}
}
