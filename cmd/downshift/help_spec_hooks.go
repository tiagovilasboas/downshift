// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import "fmt"

// hookHelp is the help for a plumbing command that a harness calls with a JSON
// event on stdin. Users never run these by hand, so the text says so and points
// at the command that does the checking instead.
func hookHelp(name, purpose string) string {
	return fmt.Sprintf(`downshift %s — %s

Usage:
  downshift %s

Flags:
  (none). Reads one JSON event on stdin. "downshift setup" writes the call into the harness.
  Fails open: on any error the subagent runs unchanged.

Examples:
  downshift try "rename the variable userId" %s

Next: downshift doctor, to check the hook is wired
`, name, purpose, name, harnessFor(name))
}

// harnessFor names the harness a hook command belongs to, for the example line.
func harnessFor(name string) string {
	switch name {
	case "claude-code", "claude-code-post-tool-use", "claude-code-subagent-stop":
		return "claude-code"
	default:
		return name
	}
}

// hookHelpTopics covers the harness hooks and the observation commands the
// hooks call after a subagent starts or ends.
var hookHelpTopics = map[string]string{
	"claude-code":               hookHelp("claude-code", "Claude Code PreToolUse hook: rewrites a subagent's model"),
	"cursor":                    hookHelp("cursor", "Cursor preToolUse hook: rewrites a subagent's model"),
	"cursor-touch":              hookHelp("cursor-touch", "Cursor session hooks: remembers the composer model for Task baseline"),
	"codex":                     hookHelp("codex", "Codex PreToolUse hook: rewrites a subagent's model"),
	"antigravity":               hookHelp("antigravity", "Antigravity PreToolUse hook: rewrites a subagent's model"),
	"kirocrew":                  hookHelp("kirocrew", "KiroCrew preToolUse hook: policy mode, exit 0 allow or 2 block"),
	"opencode":                  hookHelp("opencode", "OpenCode subagent_spawn_intent route hook (wire v1)"),
	"claude-code-post-tool-use": hookHelp("claude-code-post-tool-use", "Claude Code post-spawn observation: records what the subagent did"),
	"claude-code-subagent-stop": hookHelp("claude-code-subagent-stop", "Claude Code SubagentStop observation: records usage and verification"),
	"honor": `downshift honor — Post-spawn observation for a harness: records whether the routing was honored

Usage:
  downshift honor <harness>

Flags:
  (none). Reads one JSON event on stdin. Written by "downshift setup".
  Fails open: a read or panic prints the neutral response and exits 0.

Examples:
  downshift honor claude-code < event.json

Next: downshift session, to see the recorded decisions
`,
	"usage": `downshift usage — Post-spawn usage observation for a harness: records tokens

Usage:
  downshift usage <harness>

Flags:
  (none). Reads one JSON event on stdin. Written by "downshift setup".
  Never blocks: a read error or an oversized payload exits 0 with no stdout.

Examples:
  downshift usage claude-code < event.json

Next: downshift stats, to see the usage in the totals
`,
	"end": `downshift end — Child-end observation for a harness: marks a subagent as finished

Usage:
  downshift end <harness>

Flags:
  (none). Reads one JSON event on stdin. Written by "downshift setup".
  Records nothing for a payload that is not a child end.

Examples:
  downshift end claude-code < event.json

Next: downshift report, once tasks have finished
`,
}
