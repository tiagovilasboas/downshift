// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package claudecode

import (
	"encoding/json"

	"github.com/tiagovilasboas/downshift-ai/internal/core"
	"github.com/tiagovilasboas/downshift-ai/internal/hookport"
)

// Claude Code's PostToolUse payload carries the finished tool's result in
// tool_response, in that tool's own output shape, and the hook may answer
// hookSpecificOutput.updatedToolOutput with a value of the same shape (hooks
// reference, "PostToolUse decision control", read 2026-11-10). Bash's shape is
// documented: {stdout, stderr, interrupted, isImage, ...}. Grep's and Glob's
// are not; this adapter reads only the fields their results carry (Grep:
// content in content mode, filenames in file mode; Glob: filenames) and
// leaves every other field, counts included, exactly as received. Nothing
// here decides what to cut: internal/postrun does.

// ParseToolOutput reads a PostToolUse payload into the shared ToolOutput.
func ParseToolOutput(raw []byte) (hookport.ToolOutput, bool) {
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return hookport.ToolOutput{}, false
	}
	if ev, ok := payload["hook_event_name"].(string); ok && ev != "PostToolUse" {
		return hookport.ToolOutput{}, false
	}
	out := hookport.ToolOutput{
		Category: core.ClaudeCodeCaps.CategoryOf(stringFieldAny(payload, "tool_name", "toolName")),
		Session:  stringFieldAny(payload, "session_id", "sessionId"),
		ExitCode: -1,
	}
	if out.Session == "" {
		out.Session = sessionFromTranscript(stringFieldAny(payload, "transcript_path"))
	}
	resp, ok := payload["tool_response"].(map[string]any)
	if !ok {
		out.Opaque = true
		return out, true
	}
	out.Native = resp
	if input, ok := payload["tool_input"].(map[string]any); ok {
		out.Command, _ = input["command"].(string)
	}
	out.Stdout, out.HasStdout = resp["stdout"].(string)
	out.Stderr, out.HasStderr = resp["stderr"].(string)
	out.Content, out.HasContent = resp["content"].(string)
	out.Paths, out.HasPaths, out.Opaque = stringList(resp["filenames"])
	if code, ok := exitCodeField(resp); ok {
		out.ExitCode = code
	}
	if b, _ := resp["interrupted"].(bool); b {
		out.Opaque = true
	}
	if b, _ := resp["isImage"].(bool); b {
		out.Opaque = true
	}
	return out, true
}

// stringList reads a JSON array of strings. A missing field is (nil, false,
// false); an array holding anything but strings is opaque.
func stringList(v any) (list []string, has, opaque bool) {
	arr, ok := v.([]any)
	if !ok {
		return nil, false, v != nil
	}
	list = make([]string, 0, len(arr))
	for _, e := range arr {
		s, ok := e.(string)
		if !ok {
			return nil, false, true
		}
		list = append(list, s)
	}
	return list, true, false
}

// ReplaceToolOutput answers PostToolUse with updatedToolOutput: the tool's
// own result object with only the text fields the result already carried
// replaced, so it keeps the tool's shape. prev is kept (a systemMessage, for
// one). A result that cannot be encoded leaves prev unchanged.
func ReplaceToolOutput(prev any, out hookport.ToolOutput) any {
	if out.Native == nil {
		return prev
	}
	updated := make(map[string]any, len(out.Native))
	for k, v := range out.Native {
		updated[k] = v
	}
	if out.HasStdout {
		updated["stdout"] = out.Stdout
	}
	if out.HasStderr {
		updated["stderr"] = out.Stderr
	}
	if out.HasContent {
		updated["content"] = out.Content
	}
	if out.HasPaths {
		updated["filenames"] = out.Paths
	}
	encoded, err := json.Marshal(updated)
	if err != nil {
		return prev
	}
	answer, _ := prev.(Output)
	hso := HookSpecificOutput{HookEventName: "PostToolUse"}
	if answer.HookSpecificOutput != nil {
		hso = *answer.HookSpecificOutput
	}
	hso.UpdatedToolOutput = encoded
	answer.HookSpecificOutput = &hso
	return answer
}
