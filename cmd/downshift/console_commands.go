// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import "github.com/tiagovilasboas/downshift-ai/internal/console/shared/command"

// consoleCommands is every user-facing command the console lists. Hook
// plumbing (claude-code, cursor, honor, usage, end, …) is left out: the
// harness calls it, people do not. console_commands_test.go fails when a help
// topic has no entry here, so a new command cannot be missed.
var consoleCommands = []command.Spec{
	// Get started and fix things
	{Args: []string{"setup", "--dry-run"}, Group: "setup", Summary: "Show the diff setup would write to each harness"},
	{Args: []string{"setup"}, Group: "setup", Summary: "Wire the hooks into every harness found (asks first)", Writes: true, Terminal: true},
	{Args: []string{"setup", "--undo"}, Group: "setup", Summary: "Restore the latest backup of each hook file", Writes: true, Terminal: true},
	{Args: []string{"uninstall", "--dry-run"}, Group: "setup", Summary: "Show what uninstall would restore"},
	{Args: []string{"uninstall"}, Group: "setup", Summary: "Restore hook configs and remove wrappers", Writes: true, Terminal: true},
	{Args: []string{"doctor"}, Group: "health", Summary: "Checklist with a fix command per failure"},
	{Args: []string{"doctor", "--verbose"}, Group: "health", Summary: "Checklist with every detail"},
	{Args: []string{"config"}, Group: "health", Summary: "Every effective setting with its value and source"},
	{Args: []string{"enable"}, Group: "health", Summary: "Print shell aliases for transparent compaction"},
	{Args: []string{"version"}, Group: "health", Summary: "Print the binary version"},

	// See the effect
	{Args: []string{"session"}, Group: "session", Summary: "This session's subagents as a tree"},
	{Args: []string{"session", "--why"}, Group: "session", Summary: "The tree with the reason for each routing"},
	{Args: []string{"session", "--pause"}, Group: "session", Summary: "Pause routing for this session", Writes: true},
	{Args: []string{"session", "--resume"}, Group: "session", Summary: "Resume routing for this session", Writes: true},
	{Args: []string{"watch"}, Group: "session", Summary: "Live session tree (Ctrl-C returns here)", Terminal: true},
	{Args: []string{"accept", "--used"}, Group: "session", Summary: "Record that the delivered code was used", Writes: true},
	{Args: []string{"accept", "--discarded"}, Group: "session", Summary: "Record that the delivered code was discarded", Writes: true},
	{Args: []string{"report"}, Group: "savings", Summary: "Routed tokens per completed task, rework per verdict"},
	{Args: []string{"report", "--days=7"}, Group: "savings", Summary: "The report for the last 7 days"},
	{Args: []string{"stats"}, Group: "savings", Summary: "Routing decisions and estimated token savings"},
	{Args: []string{"stats", "--detail"}, Group: "savings", Summary: "Units, honor, quota, usage, per-session tokens"},
	{Args: []string{"stats", "--export"}, Group: "savings", Summary: "Pasteable JSON summary without prompts"},
	{Args: []string{"export"}, Group: "savings", Summary: "Counts and token totals to share (no prompts or paths)"},
	{Args: []string{"plan"}, Group: "savings", Summary: "Validate a plan from stdin and print a table", Terminal: true},
	{Args: []string{"compact", "--"}, Group: "savings", Summary: "Type a command after -- to cut its output to PASS/FAIL"},
	{Args: []string{"discover"}, Group: "savings", Summary: "Commands that ran bare but the compactor would have cut"},
	{Args: []string{"retrieve"}, Group: "savings", Summary: "Print the full output the compactor kept (retrieve <id>)"},

	// Models and quota
	{Args: []string{"models", "list"}, Group: "models", Summary: "The effective catalog (embedded or override)"},
	{Args: []string{"models", "available", "--harness"}, Group: "models", Summary: "Type a harness id: the models a spawn can use now"},
	{Args: []string{"models", "discover"}, Group: "models", Summary: "Ask each harness which models the account can use", Writes: true},
	{Args: []string{"models", "check"}, Group: "models", Summary: "Query provider APIs for new or untiered models"},
	{Args: []string{"models", "pull"}, Group: "models", Summary: "Write the user catalog.json under the state dir", Writes: true},
	{Args: []string{"route", "--harness"}, Group: "models", Summary: "Type a harness id: what would apply for one spawn"},
	{Args: []string{"quota", "status", "--harness"}, Group: "models", Summary: "Type a harness id: budget evidence and freshness"},
	{Args: []string{"gateway", "status"}, Group: "models", Summary: "Check local companion routers"},
	{Args: []string{"mcp", "serve"}, Group: "models", Summary: "Run the MCP server on stdio (Ctrl-C returns here)", Terminal: true},

	// Rules
	{Args: []string{"playbook", "list"}, Group: "rules", Summary: "The working rules a delegated worker receives"},
	{Args: []string{"playbook", "check"}, Group: "rules", Summary: "Validate the playbook"},
	{Args: []string{"playbook", "edit"}, Group: "rules", Summary: "Edit the playbook in $EDITOR", Writes: true, Terminal: true},
	{Args: []string{"playbook", "restore"}, Group: "rules", Summary: "Restore the default playbook", Writes: true},
	{Args: []string{"policy", "check"}, Group: "rules", Summary: "Validate the routing policy without routing"},

	// Experimental
	{Args: []string{"learning", "stats"}, Group: "experimental", Summary: "What the learning store has seen"},
	{Args: []string{"learning", "suggestions"}, Group: "experimental", Summary: "What the learning store suggests"},
	{Args: []string{"learning", "export"}, Group: "experimental", Summary: "Dump the learning store"},
	{Args: []string{"learning", "clear"}, Group: "experimental", Summary: "Clear the learning store", Writes: true},
	{Args: []string{"loop"}, Group: "experimental", Summary: "Autonomous loop with sensor verification", Terminal: true},
	{Args: []string{"context", "status"}, Group: "experimental", Summary: "Context optimization status and provider"},
	{Args: []string{"context", "providers"}, Group: "experimental", Summary: "Supported context optimization providers"},
	{Args: []string{"context", "doctor"}, Group: "experimental", Summary: "Diagnose context optimization providers"},
	{Args: []string{"context", "metrics"}, Group: "experimental", Summary: "Token reduction metrics from the provider"},
	{Args: []string{"context", "benchmark"}, Group: "experimental", Summary: "Compare 4 optimization scenarios"},
	{Args: []string{"context", "enable"}, Group: "experimental", Summary: "Enable a context optimization provider", Writes: true},
	{Args: []string{"context", "disable"}, Group: "experimental", Summary: "Disable context optimization", Writes: true},
	{Args: []string{"context", "compress"}, Group: "experimental", Summary: "Compress tool output from stdin", Terminal: true},

	// Maintainer
	{Args: []string{"eval-outcome", "--report"}, Group: "maintainer", Summary: "Outcome eval report (needs labelled data)"},
	{Args: []string{"probe"}, Group: "maintainer", Summary: "Record a payload's field names (as a hook)", Terminal: true},
}
