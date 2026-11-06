// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package clispec

// modelCommands are routing, models, quota and policy: what is applied to a
// spawn and why.
var modelCommands = []Command{
	{Name: "models", Summary: "The effective catalog and what each harness can use"},
	{Name: "models list", Summary: "Show the effective catalog (embedded or override)"},
	{Name: "models check", Summary: "Query provider APIs and report new or untiered models"},
	{Name: "models pull", Summary: "Write the user catalog.json under the state dir"},
	{Name: "models discover", Summary: "Ask each harness which models the account can use", Flags: f("--harness", "--dry-run", "--json")},
	{Name: "models available", Summary: "The models a spawn can use now on a harness", Flags: f("--harness", "--json")},
	{Name: "quota", Summary: "Inspect, import or collect budget evidence for a harness", Flags: f("--harness", "--source", "--ttl", "--transcript", "--model")},
	{Name: "route", Summary: "Show what would be applied for one spawn", Flags: f("--harness", "--model", "--effort", "--tier", "--agent-type", "--dir", "--json")},
	{Name: "policy", Summary: "Validate the routing policy without routing anything", Flags: f("--dry-run", "--yes|-y", "--harness")},
	{Name: "playbook", Summary: "Edit the working rules a delegated worker receives"},
	{Name: "plan", Summary: "Validate a delegation plan and print it as a table", Flags: f("--json")},
	{Name: "gateway", Summary: "Check local companion routers (9router)", Flags: f("--json", "--endpoint")},
	{Name: "mcp", Summary: "Run the MCP server the orchestrator can ask"},
	{Name: "learning", Summary: "Compressor learning status"},
	{Name: "context", Summary: "Context optimisation status, providers and compression (experimental)"},
	{Name: "context enable", Summary: "Enable the native compactor, or record a companion provider", Flags: f("--exec", "--no-exec")},
	{Name: "context compress", Summary: "Compress tool output read from stdin (experimental)", Flags: f("--exit", "--observe", "--safe", "--off")},
	{Name: "dci", Summary: "Removed: code intelligence is gone"},
	{Name: "eval-outcome", Summary: "Outcome eval: executable checks, small vs frontier pass rate", Flags: f("--verify", "--record", "--tier", "--model", "--solver", "--report", "--write", "--check", "--tasks", "--runs")},
}
