// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

// gatewayHelp covers the gateway companion routers command.
var gatewayHelp = map[string]string{
	"gateway": `downshift gateway — Inspect local companion router proxies

Usage:
  downshift gateway [status] [--json]
  downshift gateway models

Status probes local multi-provider proxy gateways (OmniRoute on :20128/v1 and 9router on :3000),
reporting reachability, endpoints, model routing prefix examples, and environment export hints.

Models explains how to format model slugs for gateway routing.

Flags:
  --json    Emit machine-readable JSON status report

Examples:
  downshift gateway
  downshift gateway status --json
  downshift gateway models

Next: downshift doctor, to verify gateway companions and harness wiring
`,
}
