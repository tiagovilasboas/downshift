// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package kirocrew

import (
	"encoding/json"
	"fmt"
)

// isShellTool reports whether name is KiroCrew's shell tool. A live payload
// (2026-10-22, audit.log) names it "shell" with tool_tags ["shell","@builtin"];
// "execute_bash" is the older name its bundled hook matchers still use.
func isShellTool(name string) bool { return name == "shell" || name == "execute_bash" }

// ShellCommand returns tool_input.command and true when this call is the
// shell tool. This implements shellwrap.AdviseAdapter.
func (ev Event) ShellCommand() (string, bool) {
	if !isShellTool(ev.ToolName) {
		return "", false
	}
	var ti map[string]any
	if json.Unmarshal(ev.ToolInput, &ti) != nil {
		return "", true
	}
	command, _ := ti["command"].(string)
	return command, true
}

// ShellBlocked is false: advising only suggests a retry, and the retried
// command still goes through KiroCrew's own permission flow.
func (ev Event) ShellBlocked(_ string) bool { return false }

// AdviseMessage is the stderr line KiroCrew relays to the model.
func (ev Event) AdviseMessage(wrappedCommand string) string {
	return fmt.Sprintf("downshift: this command prints a long log. Run it again exactly as: %s", wrappedCommand)
}
