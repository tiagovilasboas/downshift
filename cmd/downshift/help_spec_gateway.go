// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

// gatewayHelp covers the gateway companion routers command.
var gatewayHelp = map[string]string{
	"gateway": `downshift gateway — Inspect local companion router proxies

Usage:
  downshift gateway [status] [--json]
  downshift gateway on [name] [--endpoint URL]
  downshift gateway off
  downshift gateway models

On routes delegate workers (claude, codex) through the gateway (default 9router);
native harness subagents are not routed. Off turns it off.

Status probes local multi-provider proxy gateway (9router on :20128; claude gets it as is, codex gets /v1 appended),
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
