// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

// modelsHelp covers the model catalog, the quota evidence, and the offline
// replay and learning views.
var modelsHelp = map[string]string{
	"models": `downshift models — Inspect and refresh the model catalog

Usage:
  downshift models <list|check|pull|discover> [flags]

Subcommands:
  list       Show the effective catalog (embedded, or the user override)
  check      Query provider APIs and report new or untiered models
  pull       Write the user catalog.json under the state dir
  discover   Ask each harness which models the account can use

Flags:
  (see each subcommand: downshift models <name> --help)

Examples:
  downshift models list
  downshift models discover --dry-run

Next: downshift models list
`,
	"models list": `downshift models list — Show the effective model catalog

Usage:
  downshift models list

Flags:
  (none)

Examples:
  downshift models list

Next: downshift models discover, to see which of them this account can use
`,
	"models check": `downshift models check — Query provider APIs and report new or untiered models

Usage:
  downshift models check

Flags:
  (none). Makes network calls to the provider APIs.

Examples:
  downshift models check

Next: downshift models pull, to write the catalog update
`,
	"models pull": `downshift models pull — Write the user catalog.json under the state dir

Usage:
  downshift models pull

Flags:
  (none). Writes ~/.downshift/catalog.json; the embedded catalog is not changed.

Examples:
  downshift models pull

Next: downshift models list, to confirm the new entries
`,
	"models discover": `downshift models discover — Ask each harness which models the account can use

Usage:
  downshift models discover [--harness=IDS] [--dry-run] [--json]

Flags:
  --harness=IDS   Comma-separated harness ids (default: every harness with a source)
  --dry-run       Report without writing the cache
  --json          Print the result as JSON

Examples:
  downshift models discover
  downshift models discover --harness=claude-code --dry-run

Next: downshift doctor, to check the discovered models are in use
`,
	"quota": `downshift quota — Budget evidence for a harness (status, import, collect)

Usage:
  downshift quota status --harness=ID [--model=M]
  downshift quota import --harness=ID [--source=S] [--ttl=D]    (JSON on stdin, max 1 MiB)
  downshift quota collect --harness=codex --transcript=PATH
  downshift quota claude-statusline                          (statusLine bridge, reads stdin)

Flags:
  --harness=ID         Harness whose quota is observed (required except claude-statusline)
  --source=S           codex-usage, claude-statusline, cursor-usage or normalized
  --ttl=D              Observation lifetime, at most 15m (default 5m)
  --transcript=PATH    Explicit local Codex transcript (collect only)
  --model=M            Exact model to inspect (status only)

Examples:
  downshift quota status --harness=codex
  downshift quota collect --harness=codex --transcript=/path/to/rollout.jsonl

Next: downshift quota status --harness=ID, to confirm the evidence is fresh
`,
	"replay": `downshift replay — Re-decide recorded Claude Code spawns under each policy (offline)

Usage:
  downshift replay [--projects=DIR] [--policies=LIST]

Flags:
  --projects=DIR    Claude Code projects directory (default ~/.claude/projects)
  --policies=LIST   Comma-separated: current, nb_downshift, explicit_upshift,
                    nb_downshift+explicit_upshift, or economy (default current,nb_downshift)

No model call, no network, and no prompt is printed or written.

Examples:
  downshift replay
  downshift replay --policies=current,economy

Next: downshift report, for the measured numbers from real runs
`,
	"probe": `downshift probe — Maintainer tool: record a hook payload's field names (never prompts or paths)

Usage:
  downshift probe <event>

Flags:
  (none). Writes one line per call to probe.jsonl in the state dir.
  See docs/guides/probe.md.

Examples:
  downshift probe preToolUse

Next: downshift doctor, after the probe has run
`,
	"learning": `downshift learning — Show what the learning store has seen and suggested

Usage:
  downshift learning <suggestions|stats|export|clear>

Subcommands:
  suggestions   Commands that would benefit from a compressor
  stats         Learning store statistics
  export        Dump the store as JSON (shapes and counts, no output text)
  clear         Reset the learning store

Flags:
  (none)

Examples:
  downshift learning stats
  downshift learning export

Next: downshift context status, to see which compressor is active
`,
}
