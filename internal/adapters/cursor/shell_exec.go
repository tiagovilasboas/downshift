// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package cursor

import (
	"encoding/json"

	"github.com/tiagovilasboas/downshift/internal/shellwrap"
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

// ShellAdvise is the beforeShellExecution answer that carries an advice line.
// It always allows: the advice is a hint for the agent, never a block.
func ShellAdvise(message string) Output {
	return Output{Permission: "allow", AgentMessage: message}
}
