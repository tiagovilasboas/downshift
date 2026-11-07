// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package claudecode

import (
	"encoding/json"
	"errors"
)

// shellToolName is Claude Code's shell tool.
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

// ShellRunsInBackground reports whether tool_input.run_in_background is set.
// Any present value other than false or null counts as set, so an odd value
// leaves the command as written.
func (ev Event) ShellRunsInBackground() bool {
	var ti map[string]json.RawMessage
	if json.Unmarshal(ev.ToolInput, &ti) != nil {
		return false
	}
	raw, ok := ti["run_in_background"]
	if !ok {
		return false
	}
	switch string(raw) {
	case "false", "null":
		return false
	}
	return true
}

// promptingModes are the permission modes in which Claude Code still asks the
// user before a tool call that no allow rule covers. An empty mode (an older
// payload without the field) is the default mode.
var promptingModes = map[string]bool{"": true, "default": true, "plan": true, "acceptEdits": true}

// ShellPromptsSuppressed reports a permission mode in which Claude Code runs
// shell calls without asking (bypassPermissions, dontAsk). Any mode this
// adapter does not know counts as one, so a new mode fails safe.
func (ev Event) ShellPromptsSuppressed() bool { return !promptingModes[ev.PermissionMode] }

// shellOutput is the PreToolUse answer that replaces the shell command. It
// carries no permissionDecision on purpose: the harness's own permission flow
// (allow, ask and deny rules) then applies to the rewritten command.
type shellOutput struct {
	HookSpecificOutput shellHookOutput `json:"hookSpecificOutput"`
}

type shellHookOutput struct {
	HookEventName string          `json:"hookEventName"`
	UpdatedInput  json.RawMessage `json:"updatedInput"`
}

// ShellRewrite returns the hook output that sets tool_input.command to
// command. Every sibling field (description, timeout, run_in_background and
// any future field) is copied through with its value unchanged.
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
	return shellOutput{HookSpecificOutput: shellHookOutput{HookEventName: "PreToolUse", UpdatedInput: updated}}, nil
}
