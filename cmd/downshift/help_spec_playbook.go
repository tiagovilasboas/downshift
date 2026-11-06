// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

// playbookHelp covers the working rules a delegated worker receives.
var playbookHelp = map[string]string{
	"playbook": `downshift playbook — Edit the working rules a delegated worker receives

Usage:
  downshift playbook list
  downshift playbook show <name>
  downshift playbook edit <name>
  downshift playbook add <name>
  downshift playbook remove <name>
  downshift playbook restore <name>
  downshift playbook save <name>   (text on stdin)
  downshift playbook check

A playbook is a markdown set handed to a worker verbatim. General always
applies; a delegation may name one more playbook for its area (Backend,
Frontend or your own). Names match regardless of case and accents. Files live
in <state dir>/playbooks/<id>.md, with "# Name" on the first line. Rules are
instructions to an agent, never enforced, and they travel with every task of
their area, so keep them short: list and check warn above 8000 characters.

edit and add open the file in $VISUAL or $EDITOR, then check it. remove drops
a playbook (a shipped one stays removed until restore); General cannot be
removed. restore puts a shipped playbook back to the text that ships.

Flags:
  none

Examples:
  downshift playbook list
  downshift playbook edit backend
  downshift playbook add "Banco de Dados"

Next: downshift doctor, to check the rest of the install
`,
	"plan": `downshift plan — Validate a delegation plan and print it as a table

Usage:
  downshift plan [--json] < plan.json

Flags:
  --json   Print the filled plan as JSON

Reads {"goal", "parts":[{id, title, agent, points, depends_on, files, worker,
playbook}]} on stdin. Checks ids, agents, points (1, 2, 3 or 5; split larger
parts), dependencies and cycles, then fills tier, playbook, wave and route.
Exits 1 and lists every problem on invalid input. It calls no model.

Examples:
  echo '{"goal":"g","parts":[{"id":"a","agent":"coder","points":3}]}' | downshift plan

Next: docs/guides/plan.md
`,
}
