// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package cursor

// Shell wrap stub — awaiting live payload confirmation.
//
// To activate in-place command replacement for Cursor:
//  1. Capture a live PreToolUse payload with Cursor's shell tool and confirm
//     that updated_input.command changes the command the harness runs.
//     Record the evidence in docs/evidence/cursor-shell-payload-YYYY-MM-DD.json.
//  2. Set shellToolName to the confirmed tool_name.
//  3. Implement ShellRewrite the same way claudecode/shell.go does.
//  4. Set CanRewriteShellCommand: true in core.CursorCaps.
//
// Until that evidence exists, ShellCommand always returns false so Handle
// never enters the rewrite path, regardless of the capability flag.

// ShellCommand always returns false until the shell tool name is confirmed
// from a live payload. This implements shellwrap.RewriteAdapter as a stub.
func (ev Event) ShellCommand() (string, bool) { return "", false }

// ShellRewrite is unreachable while ShellCommand returns false.
func (ev Event) ShellRewrite(_ string) (any, error) { return nil, nil }

// ShellBlocked always returns false (no deny/ask rules apply to shell in this stub).
func (ev Event) ShellBlocked(_ string) bool { return false }
