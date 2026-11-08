// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package cursor

import (
	"encoding/json"

	"github.com/tiagovilasboas/downshift-ai/internal/shellwrap"
)

// shellExecEvent is the hook_event_name of Cursor's beforeShellExecution hook
// (cursor.com/docs/agent/hooks, read 2026-10-24).
const shellExecEvent = "beforeShellExecution"

// ShellExec is the beforeShellExecution payload: the full command the agent is
// about to run. It has no tool envelope, so it is a separate type from Event.
type ShellExec struct {
	HookEventName string `json:"hook_event_name"`
	Command       string `json:"command"`
	Cwd           string `json:"cwd,omitempty"`
	Sandbox       bool   `json:"sandbox,omitempty"`
}

// ParsePayload reads one payload the cursor hook receives. A
// beforeShellExecution payload yields a ShellExec; any other payload is the
// preToolUse Event. Invalid JSON is an error, which the runner fails open.
func ParsePayload(data []byte) (any, error) {
	var head struct {
		HookEventName string `json:"hook_event_name"`
	}
	if err := json.Unmarshal(data, &head); err != nil {
		return nil, err
	}
	if head.HookEventName == shellExecEvent {
		var ev ShellExec
		err := json.Unmarshal(data, &ev)
		return ev, err
	}
	var ev Event
	err := json.Unmarshal(data, &ev)
	return ev, err
}

// ShellCommand returns the command about to run. A beforeShellExecution call
// is always a shell call.
func (ev ShellExec) ShellCommand() (string, bool) { return ev.Command, true }

// ShellBlocked is false: Cursor keeps no readable allow/deny file for shell,
// so there is no rule to honor. Cursor's own permission flow still applies.
func (ev ShellExec) ShellBlocked(_ string) bool { return false }

// AdviseMessage is the agent_message Cursor relays to the model.
func (ev ShellExec) AdviseMessage(wrappedCommand string) string {
	return shellwrap.AdviseText(wrappedCommand)
}

// IsShellCall reports whether a ParsePayload result may be a shell command: a
// beforeShellExecution payload, a preToolUse call of the Shell tool, or a call
// that names no tool (an unknown event name, a missing one). Only a call that
// names another tool is known not to be a shell command, so only it may get
// an allow.
func IsShellCall(parsed any) bool {
	switch p := parsed.(type) {
	case ShellExec:
		return true
	case Event:
		return p.ToolName == shellToolName || p.ToolName == ""
	}
	return false
}

// ShellAdviseOutput is the beforeShellExecution answer that carries an advice
// line. It has no permission field on purpose: the advice is a hint for the
// agent, and Downshift never approves a command it does not own, so Cursor's
// own approval flow decides.
type ShellAdviseOutput struct {
	AgentMessage string `json:"agent_message"`
}

// ShellAdvise builds the advice-only beforeShellExecution answer.
func ShellAdvise(message string) ShellAdviseOutput {
	return ShellAdviseOutput{AgentMessage: message}
}
