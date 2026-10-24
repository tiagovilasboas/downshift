// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package codex

import (
	"encoding/json"
	"errors"
)

// shellToolName is how Codex reports its shell tool to PreToolUse hooks. The
// unified exec tool (exec_command) is reported under this same canonical name,
// so one name covers both (Codex hooks documentation, PreToolUse).
const shellToolName = "Bash"

// ShellCommand returns tool_input.command and true when this call is the
// shell tool. The shared runner decides whether to wrap it; the adapter only
// parses.
func (ev Event) ShellCommand() (string, bool) {
	if ev.ToolName != shellToolName {
		return "", false
	}
	var ti map[string]any
	if json.Unmarshal(ev.ToolInput, &ti) != nil {
		return "", true
	}
	command, _ := ti["command"].(string)
	return command, true
}

// shellOutput is the PreToolUse answer that replaces the shell command. Codex
// only honors updatedInput together with permissionDecision "allow".
type shellOutput struct {
	HookSpecificOutput shellHookOutput `json:"hookSpecificOutput"`
}

type shellHookOutput struct {
	HookEventName      string          `json:"hookEventName"`
	PermissionDecision string          `json:"permissionDecision"`
	UpdatedInput       json.RawMessage `json:"updatedInput"`
}

// ShellRewrite returns the hook output that sets tool_input.command to
// command. Every sibling field of tool_input is copied through unchanged.
func (ev Event) ShellRewrite(command string) (any, error) {
	var ti map[string]json.RawMessage
	if err := json.Unmarshal(ev.ToolInput, &ti); err != nil {
		return nil, err
	}
	if ti == nil {
		return nil, errors.New("tool_input is not an object")
	}
	encoded, err := json.Marshal(command)
	if err != nil {
		return nil, err
	}
	ti["command"] = encoded
	updated, err := json.Marshal(ti)
	if err != nil {
		return nil, err
	}
	return shellOutput{HookSpecificOutput: shellHookOutput{
		HookEventName:      "PreToolUse",
		PermissionDecision: "allow",
		UpdatedInput:       updated,
	}}, nil
}

// ShellBlocked is always false: Codex has no settings-level deny or ask rules
// for shell commands that the adapter could read, so there is nothing to
// defer to. The shared shellwrap policy still refuses metacharacters, the
// allowlist misses and the kill switches.
func (ev Event) ShellBlocked(_ string) bool { return false }
