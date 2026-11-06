// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package shellwrap

// RewriteAdapter is implemented by adapters whose harness supports in-place
// command replacement (updatedInput / updated_input). An adapter that
// implements this interface but whose harness has CanRewriteShellCommand false
// in the capability table is a no-op: Handle returns a passthrough Result.
//
// Claude Code implements this today. Cursor and Codex are candidates once a
// live payload proves that updated_input.command is honored.
type RewriteAdapter interface {
	// ShellCommand returns the command and true when this call is the
	// harness's shell tool. The shared runner never routes shell calls.
	ShellCommand() (command string, isShell bool)
	// ShellRewrite returns the wire output that replaces the command,
	// copying every sibling field of tool_input unchanged.
	ShellRewrite(command string) (any, error)
	// ShellBlocked reports whether the harness's own deny/ask rules forbid
	// rewriting this command. Any rule-read error returns true (fail closed).
	ShellBlocked(command string) bool
}

// AdviseAdapter is implemented by adapters whose harness cannot rewrite in
// place but can show the model one line of stderr and exit 2 to trigger a
// retry. KiroCrew uses this path today.
//
// Any harness whose hook protocol is exit-0/exit-2 with no updated_input can
// implement AdviseAdapter to participate in shell wrap without protocol changes.
type AdviseAdapter interface {
	// ShellCommand returns the command and true when this call is the
	// harness's shell tool.
	ShellCommand() (command string, isShell bool)
	// ShellBlocked reports whether the harness's own rules forbid advising
	// this command. Any rule-read error returns true (fail closed).
	ShellBlocked(command string) bool
	// AdviseMessage returns the stderr line to show the model. The shared
	// runner computes the wrapped command string; the adapter only formats the
	// human-readable suggestion around it.
	AdviseMessage(wrappedCommand string) string
}
