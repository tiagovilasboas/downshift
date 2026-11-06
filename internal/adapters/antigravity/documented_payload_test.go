// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package antigravity

import (
	"encoding/json"
	"testing"
)

// The vendor hooks page (see docs/evidence/vendor-hook-docs-2026-10-11.md)
// documents these camelCase PreToolUse fields. Unknown documented fields must
// not break parsing, and the project dir must come from workspacePaths.
func TestEvent_ParsesDocumentedPreToolUseFields(t *testing.T) {
	raw := `{
	  "toolCall": {"name": "invoke_subagent", "args": {"Subagents": [{"Prompt": "rename a variable"}]}},
	  "stepIdx": 3,
	  "conversationId": "c-1",
	  "workspacePaths": ["/work/proj"],
	  "transcriptPath": "/tmp/t.jsonl",
	  "artifactDirectoryPath": "/tmp/a",
	  "modelName": "parent-model"
	}`
	var ev Event
	if err := json.Unmarshal([]byte(raw), &ev); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if ev.ToolCall.Name != "invoke_subagent" {
		t.Errorf("tool name = %q", ev.ToolCall.Name)
	}
	if got := ev.ProjectDir(); got != "/work/proj" {
		t.Errorf("ProjectDir = %q, want /work/proj", got)
	}
	if got := ev.TaskText(); got != "rename a variable" {
		t.Errorf("TaskText = %q", got)
	}
}
