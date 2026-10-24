// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

// experimentalHelp covers the context-optimisation and code-intelligence
// commands. Both are kept but may change, so the text stays close to the code.
var experimentalHelp = map[string]string{
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
	"dci": `downshift dci — Code intelligence: inject relevant code context into subagent prompts (experimental)

Usage:
  downshift dci <status|enable|disable|doctor|inspect|impact|benchmark|hook>

Subcommands:
  status      DCI mode, index cache and statistics
  enable      Enable (active: inject context; observe: telemetry only)
  disable     Turn DCI off
  doctor      Diagnose health, file limits, permissions and budgets
  inspect     Simulate retrieval and context for a query
  impact      Show what a symbol or file touches
  benchmark   Latency, relevance and budget benchmarks
  hook        Run as a harness hook (reads stdin)

Flags:
  (see each subcommand: downshift dci <name> --help, for enable and inspect)

Examples:
  downshift dci status
  downshift dci enable observe

Next: downshift dci doctor, if the mode is not what you set
`,
	"dci enable": `downshift dci enable — Turn DCI on, in active or observe mode

Usage:
  downshift dci enable [active | observe] [--budget=N]

Flags:
  --budget=N   Context budget in bytes, 512 to 16384 (default 3500)

Examples:
  downshift dci enable observe
  downshift dci enable active --budget=6000

Next: downshift dci doctor
`,
	"dci inspect": `downshift dci inspect — Simulate retrieval and context generation for a query

Usage:
  downshift dci inspect [--json] <prompt>

Flags:
  --json   Machine-readable output

Examples:
  downshift dci inspect "where is the session tree rendered"

Next: downshift dci enable, once the retrieved context looks right
`,
	"dci impact": `downshift dci impact — Show what a symbol or file touches

Usage:
  downshift dci impact [--json] <symbol_or_file>

Flags:
  --json   Machine-readable output

Examples:
  downshift dci impact internal/core/route.go

Next: downshift dci inspect, to see the context it would give a subagent
`,
	"dci hook": `downshift dci hook — Run DCI as a harness hook, reading the event on stdin

Usage:
  downshift dci hook <harness>

Flags:
  (none). Written by "downshift setup" when DCI is active.

Examples:
  downshift dci hook claude-code < event.json

Next: downshift dci doctor
`,
}
