// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

// policyHelp covers the routing policy file check.
var policyHelp = map[string]string{
	"policy": `downshift policy — Validate the routing policy without routing anything

Usage:
  downshift policy check
  downshift policy init [--dry-run] [--yes]
  downshift policy economy on|off [--user] [--dry-run] [--yes]
  downshift policy apply --harness claude-code [--dry-run] [--yes]

Reads the nearest project .downshift/policy.json (searched upward from the
current directory) and the user's policy.json in the state directory, and
checks the routing section of each. The tier floor in the same files is
checked too. Exits 0 when both are usable; exits 1 with the first refusal
(file, field and problem) otherwise. A missing file is fine: no profiles and
no rules mean the requested model is kept.

Flags:
  none

policy economy turns economy mode on or off by writing the top-level "mode"
key ("economy" or "standard") to the nearest project .downshift/policy.json,
or to the user's policy.json with --user. Downshift has no built-in agent-type
table any more (the orchestrator chooses the model), so economy mode acts only
on the policy you declared. It asks before writing; --dry-run
shows the change; --yes skips the question.

policy init creates .downshift/policy.json in the current directory with no
routing policy. It asks before writing; --dry-run shows the file and writes
nothing; --yes skips the question. An existing file is left untouched.
downshift setup offers the same file, unless --no-policy is passed.

policy apply compiles routing.profiles declared in policy.json into harness-native
agent definitions. For --harness claude-code, it writes or updates
<project>/.claude/agents/<name>.md, where <project> is the directory that holds the
nearest .downshift/policy.json (not the directory you ran it from). An existing file
keeps its mode and everything but its model line. A new file gets name, description
and model in its frontmatter, and a body: the description comes from the profile's
"description" and is never invented, so a profile without one cannot be created.
A policy that does not load is not applied, and a symlinked target is never written.
Either every file is written or none is. It asks before writing; --dry-run shows the
change; --yes skips the question. An automatic backup is created before modifying
existing files.

Examples:
  downshift policy check
  downshift policy init --dry-run
  downshift policy economy on --dry-run
  downshift policy apply --harness claude-code --dry-run

Next: downshift config, to see every effective setting and where it comes from
`,
}
