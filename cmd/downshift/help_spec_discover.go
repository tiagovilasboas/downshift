// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

// discoverHelp covers the command that finds commands which ran bare.
var discoverHelp = map[string]string{
	"discover": `downshift discover — Find commands the compactor would have changed but that ran bare

Usage:
  downshift discover [--since 7d] [--harness claude-code|codex|all] [--top N] [--json]

Flags:
  --since D      Window to scan: 7d, 36h, 90m (default 7d)
  --harness H    claude-code, codex or all (default all)
  --top N        How many missed command groups to list (default 10)
  --json         Print the report as JSON

Reads the harness transcripts in memory and writes nothing. The report holds
command families, program names and counts or sizes, never arguments or paths.
Result sizes for Codex commands that share one call are split evenly.

Examples:
  downshift discover
  downshift discover --since 3d --harness claude-code --json

Next: downshift setup, or downshift enable, to route those commands through the compactor
`,
}
