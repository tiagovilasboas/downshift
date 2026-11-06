// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package clispec

// dailyCommands are the commands a person types: setup, the reports and the
// compactor.
var dailyCommands = []Command{
	{Name: "setup", Summary: "Wire the hooks into every harness found (backup, diff, --undo)", Flags: f("--tui", "--no-tui", "--dry-run", "--undo", "--yes|-y", "--no-session-policy", "--you-should-know", "--no-plugin", "--no-mods", "--no-mcp", "--no-skills", "--no-steering", "--no-policy", "--keep-binaries", "--workers", "--allow-unsafe-binary", "!--mcp")},
	{Name: "uninstall", Summary: "Restore harness hook configurations and remove wrappers", Flags: f("--dry-run", "--yes|-y", "--purge")},
	{Name: "accept", Summary: "Record developer acceptance for code delivered by subagents", Flags: f("--session", "--used", "--discarded")},
	{Name: "loop", Summary: "Run an engineering loop with sensor verification and cascade auto-upgrade", Flags: f("--task", "--sensor", "--iterations", "--timeout", "--session", "--model", "--batch", "--continue-on-error", "--json")},
	{Name: "doctor", Summary: "Checklist: binary, hooks per harness, state, with a fix per failure", Flags: f("--verbose", "--json", "--quiet", "--check-latest")},
	{Name: "stats", Summary: "Decisions and the tiers spawns ran on", Flags: f("--days", "--by-harness", "--detail", "--export")},
	{Name: "report", Summary: "Measured: routed spawns, tokens per completed task, rework per verdict", Flags: f("--days", "--json", "--text", "--markdown", "--daily", "--history", "--by")},
	{Name: "report clear", Summary: "Move every measurement log to a backup so the measurement starts from zero", Flags: f("--dry-run", "--yes|-y")},
	{Name: "discover", Summary: "Find commands the compactor would have changed but that ran bare", Flags: f("--since", "--harness", "--top", "--json")},
	{Name: "export", Summary: "Counts and token totals an outside operator can share", Flags: f("--days")},
	{Name: "session", Summary: "This session's subagents as a tree (every harness)", Flags: f("--tree", "--why", "--pause", "--resume", "--session", "--json", "--panel", "--no-report", "--brief")},
	{Name: "watch", Summary: "Live session tree, redrawn every interval", Flags: f("--session", "--interval", "--once")},
	{Name: "config", Summary: "Every effective setting with its value and source", Flags: f("--json")},
	{Name: "enable", Summary: "Print shell aliases for transparent compaction"},
	{Name: "shell-init", Summary: "Deprecated alias of enable", AliasOf: "enable", Legacy: true},
	{Name: "compact", Summary: "Cut a test, build or status command's output to its PASS/FAIL summary", Flags: f("--session", "--allowlisted", "--discard-stderr", "--exit-zero")},
	{Name: "compactor", Summary: "Product name of compact", AliasOf: "compact", Legacy: true},
	{Name: "exec", Summary: "Old name of compact", AliasOf: "compact", Legacy: true},
	{Name: "retrieve", Summary: "Print the full output the compactor kept for an id", Flags: f("--grep", "--head", "--tail", "--lines")},
	{Name: "version", Summary: "Print the binary version"},
	{Name: "--version", Summary: "Print the binary version", AliasOf: "version", Legacy: true},
	{Name: "help", Summary: "Show the common commands, or every command with --all", Flags: f("--all|-a")},
	{Name: "--help", Summary: "Show the common commands", AliasOf: "help", Legacy: true},
	{Name: "-h", Summary: "Show the common commands", AliasOf: "help", Legacy: true},
	{Name: "docs-manifest", Summary: "Print the documentation manifest as JSON", Hidden: true},
}
