// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
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
