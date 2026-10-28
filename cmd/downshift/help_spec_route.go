// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

// routeHelp covers `downshift route`, the dry-run of what the hook applies.
var routeHelp = map[string]string{
	"route": `downshift route — Show what Downshift would apply for one spawn

Usage:
  downshift route --harness=ID [--model=ID] [--effort=E] [--tier=T] [--agent-type=T] [--dir=DIR] [--json]

Flags:
  --harness=ID      Harness the spawn runs on (required)
  --model=ID        The model you chose; it is the request. Unknown ids are kept as given
  --effort=E        Requested effort: low, medium or high (echoed when nothing moves)
  --tier=T          Tier hint: small, mid or frontier (only moves down, never up)
  --agent-type=T    Subagent type, matched against declared profiles and rules
  --dir=DIR         Project directory holding .downshift/policy.json (default: current directory)
  --json            Print the result as JSON (schema_version 1)

Examples:
  downshift route --harness=claude-code --model=MODEL_ID
  downshift route --harness=claude-code --model=MODEL_ID --tier=small --json

Downshift applies the policy you declared on top of your choice; it never picks
a model of its own. No prompt text is accepted or printed.

Next: downshift policy check, to validate the policy file this reads
`,
}
