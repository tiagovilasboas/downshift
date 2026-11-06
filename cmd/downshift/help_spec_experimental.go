// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

// experimentalHelp covers the context-optimisation commands, which may change,
// so the text stays close to the code, and the notice of the removed dci command.
var experimentalHelp = map[string]string{
	"dci": `downshift dci — Removed: code intelligence is gone

Usage:
  downshift dci [anything]

Code intelligence (DCI) was removed. The command stays only so a harness
configured by an older setup keeps working: ` + "`downshift dci hook <harness>`" + ` exits 0
with no output, and every other form prints a one-line notice and exits 0.

Flags:
  none

Examples:
  downshift dci hook claude-code   # silent, exit 0

Next: downshift setup, to write the current hooks
`,
	"context": `downshift context — Context optimisation status, providers and compression (experimental)

Usage:
  downshift context <status|providers|doctor|enable|disable|metrics|compress|benchmark>

Subcommands:
  status      Current context optimisation status and active provider
  providers   Supported companion providers
  doctor      Diagnostics for the providers and harnesses
  enable      Enable the native provider (default) or record a companion (headroom)
  disable     Clear one companion record (native compact cannot be disabled)
  metrics     Command metrics, and bytes saved by downshift compact
  compress    Compress stdin tool output (mode: observe, safe or off)
  benchmark   Scenario matrix and measured compressor bytes

Flags:
  (see each subcommand: downshift context <name> --help, for enable and compress)

Examples:
  downshift context status
  downshift context metrics

Next: downshift context doctor, if a provider is not active
`,
	"context enable": `downshift context enable — Enable the native compactor, or record a companion provider

Usage:
  downshift context enable [provider] [--exec | --no-exec]

Flags:
  --exec      Add the shell wrapper (native provider only)
  --no-exec   Leave the shell wrapper off (native provider only)

provider defaults to "native". "headroom" records the companion.

Examples:
  downshift context enable
  downshift context enable native --no-exec

Next: downshift context doctor
`,
	"context compress": `downshift context compress — Compress tool output read from stdin (experimental)

Usage:
  downshift context compress [observe | safe | off]

Flags:
  (none). The mode is a positional argument: observe, safe or off.

Examples:
  go test ./... 2>&1 | downshift context compress safe

Next: downshift context metrics, for the bytes saved
`,
}
