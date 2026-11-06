// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package cursor

import (
	"encoding/json"
	"errors"
)

// shellToolName is Cursor's shell tool as a preToolUse matcher value
// (cursor.com/docs/agent/hooks, read 2026-10-23).
const shellToolName = "Shell"

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

// shellOutput is the preToolUse answer that replaces the shell command. It
// carries no permission field on purpose, so Cursor's own permission flow
// applies to the rewritten command.
type shellOutput struct {
	UpdatedInput json.RawMessage `json:"updated_input"`
}

// ShellRewrite returns the hook output that sets tool_input.command to
// command. Every sibling field is copied through unchanged.
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
	return shellOutput{UpdatedInput: updated}, nil
}

// ShellBlocked is false: Cursor keeps no settings file of allow/deny rules
// that Downshift can read, so there is nothing to check. Cursor's own
// permission flow still applies to the rewritten command.
func (ev Event) ShellBlocked(_ string) bool { return false }
