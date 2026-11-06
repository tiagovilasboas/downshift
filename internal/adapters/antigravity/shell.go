// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package antigravity

import (
	"encoding/json"
)

// shellToolName and shellArg are the Antigravity shell tool and its command
// argument as the vendor hooks page documents them (antigravity.google/docs/hooks,
// PreToolUse on run_command, arg CommandLine). Documented, not live-verified.
const (
	shellToolName = "run_command"
	shellArg      = "CommandLine"
)

// ShellCommand returns args.CommandLine and true when this call is the shell
// tool. The shared runner decides whether to wrap it; the adapter only parses.
func (ev Event) ShellCommand() (string, bool) {
	if ev.ToolCall.Name != shellToolName {
		return "", false
	}
	var args map[string]any
	if json.Unmarshal(ev.ToolCall.Args, &args) != nil {
		return "", true
	}
	command, _ := args[shellArg].(string)
	return command, true
}

// ShellRewrite returns the hook output that replaces CommandLine. The vendor
// merges overwrite shallowly into toolCall.args, so it carries only the key
// that changes and every sibling argument stays as the harness sent it.
func (ev Event) ShellRewrite(command string) (any, error) {
	overwrite, err := json.Marshal(map[string]string{shellArg: command})
	if err != nil {
		return nil, err
	}
	return Output{Decision: "allow", Overwrite: overwrite}, nil
}

// ShellBlocked is false: Antigravity has no settings file of allow/deny rules
// that Downshift can read (UnknownUserRules limits the rewrite to the
// read-only set instead).
func (ev Event) ShellBlocked(_ string) bool { return false }
