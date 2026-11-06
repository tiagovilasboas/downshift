// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

// dailyHelp covers the commands a user runs in the install, check, use, fix
// loop, plus the help and version commands themselves.
var dailyHelp = map[string]string{
	"setup": `downshift setup — Wire the hooks into every harness found (backup, diff, --undo)

Usage:
  downshift setup [harness...] [--dry-run] [--undo] [--yes] [--tui] [--no-tui] [--no-mcp]

Flags:
  --tui                  Open the setup wizard even when stdin is not a terminal
  --no-tui               Plain prompts even in a terminal
  --dry-run              Show the diff, write nothing
  --undo                 Restore the latest backup of each hook file
  --yes, -y              Do not ask before writing
  --no-session-policy    Wire the hooks only; skip the Claude Code main-session policy
  --you-should-know      Also turn on Claude Code's "You should know" mod (opt-in, billed)
  --no-plugin            Claude Code: keep hooks in settings.json instead of the plugin
  --no-mods              Claude Code: skip the downshift-agents mod (band and pane)
  --no-mcp               Do not register the MCP server in Claude Code and Codex (on by default)
  --allow-unsafe-binary  Accept a binary in a temporary or world-writable directory

In a terminal, setup opens the setup wizard: it shows the diff and asks before
writing. --yes, --dry-run and --undo skip the wizard (scripts, CI and agents).

Examples:
  downshift setup
  downshift setup claude-code
  downshift setup --dry-run

Next: downshift doctor
`,
	"uninstall": `downshift uninstall — Restore harness hook configurations and remove wrappers

Usage:
  downshift uninstall [--dry-run] [--yes] [--purge]

Flags:
  --dry-run   Show what would be restored, write nothing
  --yes, -y   Do not prompt before restoring
  --purge     Also remove ~/.downshift state dir and purge all backup files

Examples:
  downshift uninstall --dry-run
  downshift uninstall --yes

Next: downshift setup, if you want to reinstall
`,
	"accept": `downshift accept — Record developer acceptance for code delivered by subagents

Usage:
  downshift accept [--session=<id>] [--used | --discarded]

Flags:
  --session=<id>  Session hash to record acceptance for (default: most recent session)
  --used          Mark session outputs as accepted and kept (default)
  --discarded     Mark session outputs as reverted or rewritten

Examples:
  downshift accept --used
  downshift accept --session=sess_123 --discarded

Next: downshift stats
`,
	"loop": `downshift loop — Run autonomous engineering loop with sensor verification and cascade auto-upgrade

Usage:
  downshift loop [--task=<desc>] [--sensor=<cmd>] [--iterations=<n>] [--timeout=<dur>] [--session=<id>] [--batch=<tasks.json>] [--continue-on-error] [--json] [-- <sensor-cmd>]

Flags:
  --task=<desc>      Task description or goal
  --sensor=<cmd>     Sensor command to run for verification (or after --)
  --iterations=<n>   Maximum iterations before tripwire stop (default: 3)
  --timeout=<dur>    Maximum duration for the loop (default: 5m)
  --session=<id>     Session ID for telemetry and cascade tracking
  --batch=<file>     Run the tasks of a JSON array [{"task","sensor","model"}] in order;
                     stops at the first failure, the rest are reported as skipped
  --continue-on-error  With --batch, keep going after a failed task
  report             Acceptance summary (used / discarded) of loop and accept runs,
                     optional --session=<id> and --json; run as: downshift loop report
  --json             Print result as JSON

Examples:
  downshift loop --batch=tasks.json --continue-on-error --json
  downshift loop --task="Add healthcheck" -- go test ./...
  downshift loop --task="Fix auth race" --sensor="go test -race ./..."
  downshift loop --iterations=5 -- make check

Next: downshift stats, to view session token savings and acceptance
`,
	"doctor": `downshift doctor — Checklist: binary, hooks per harness, state, with a fix per failure

Usage:
  downshift doctor [--verbose] [--json]

Flags:
  --verbose   List every model without a verified efforts list, and more detail
  --json      Machine-readable checklist

Examples:
  downshift doctor
  downshift doctor --json

Next: open a new harness session, then run "downshift session"
`,
	"stats": `downshift stats — Decisions and the tiers spawns ran on

Usage:
  downshift stats [--days=N] [--by-harness] [--detail | --export]

Flags:
  --days=N        Look back N days (default 30; 0 = all time)
  --by-harness    Markdown table: spawns, tier and effort per harness
  --detail        Full report: units, honor, quota, usage, per-session tokens
  --export        Print a pasteable JSON summary without prompts

Examples:
  downshift stats
  downshift stats --days=7 --detail

Next: downshift report, for the rework and tokens per completed task of routed spawns
`,
	"report": `downshift report — Measured: routed spawns, tokens per completed task, rework per verdict

Usage:
  downshift report [--days=N] [--json|--text]
  downshift report clear [--dry-run] [--yes]

Flags:
  --days=N   Look back N days (default 30)
  --json     Machine-readable output
  --text     Plain columns for a terminal (same figures)

The default output is markdown: headings and tables a harness chat renders.
The Compactor section breaks down, per harness, each command family, why a
run was not compacted and why the hook left a command unwrapped.

clear moves every measurement log (routing, child-end, compaction, context and
compressor learning) to backups/report-clear-<time> in the state directory, so
the report starts again from zero. Config, policy, session models, model
discovery and quota are kept. It asks first; --yes skips the question and
--dry-run lists the files without moving them.

Examples:
  downshift report
  downshift report --days=14 --json
  downshift report --text
  downshift report clear --dry-run

Next: downshift session, to see one run as a tree
`,
	"export": `downshift export — Counts and token totals an outside operator can share

Usage:
  downshift export [--days=N]

Flags:
  --days=N   Look back N days (default 30; 0 = all time)

The one artefact an outside operator shares after a measurement week. It prints
JSON with counts and token totals only: no prompts, file paths, session ids,
model names, hostnames or usernames. It refuses to print anything a strict
allowlist of field names and string values does not pass, so nothing private
leaves the machine. (This is a different command from downshift learning
export, which dumps the local compressor-learning store.)

Examples:
  downshift export
  downshift export --days=7

Next: downshift report, for the measured rework per routing verdict
`,
	"session": `downshift session — This session's subagents as a tree (every harness)

Usage:
  downshift session [--tree | --why | --pause | --resume | --panel [--no-report] [--brief]] [--session=ID] [--json]

Flags:
  --tree          The subagents as a tree (default)
  --why           Why the latest decision was made, in plain words
  --pause         Classify and log only, for up to 12h
  --resume        Undo --pause
  --session=ID    Session id (default: the harness's own, else the most recent)
  --json          The tree as JSON
  --panel         Per-spawn state and session savings as JSON, for a panel mod
  --no-report     With --panel: skip the 30-day report (fast)
  --brief         With --panel: only the fields the agents band draws

Examples:
  downshift session
  downshift session --why

Next: downshift watch, for the live view
`,
	"watch": `downshift watch — Live session tree, redrawn every interval (Ctrl-C to quit)

Usage:
  downshift watch [--session=ID] [--interval=1s] [--once]

Flags:
  --session=ID     Session id (default: the harness's own, else the most recent)
  --interval=D     Redraw interval, at least 200ms (default 1s)
  --once           Draw one frame without clearing the screen, then exit

Examples:
  downshift watch
  downshift watch --once

Next: downshift session --why, for the latest decision
`,
	"config": `downshift config — Every effective setting with its value and source

Usage:
  downshift config [--json]

Flags:
  --json   Machine-readable output

Examples:
  downshift config
  downshift config --json

Next: downshift doctor, if a value looks wrong
`,
	"enable": `downshift enable — Print shell aliases for transparent compaction (eval in .bashrc/.zshrc)

Usage:
  downshift enable

Flags:
  (none). DOWNSHIFT_SHELL_ALIASES=0 turns the aliases off.
  "shell-init" is a deprecated alias of this command.

Examples:
  eval "$(downshift enable)"    # add this line to ~/.zshrc or ~/.bashrc

Next: downshift doctor, to check the shell helper loads
`,
	"compact": `downshift compact — Cut the output of a test, build or status command to its PASS/FAIL summary

Usage:
  downshift compact [--session=HASH] [--allowlisted] [--discard-stderr] [--exit-zero] -- <program> [args...]

Flags:
  --session=HASH   Attribute the saved bytes to this session (set by the hook)
  --allowlisted    Only ask whether the command is allowlisted: exit 0 yes, 1 no
  --discard-stderr Send the program's stderr to the null device (the hook's 2>/dev/null)
  --exit-zero      Exit 0 and print the real code when it is not 0 (the hook's || true)

The real exit code is kept. Only clean output of an allowlisted family is cut.
Set DOWNSHIFT_EXEC=0 to run the program as a plain passthrough.
"exec" and "compactor" are older names for this command.

Examples:
  downshift compact -- go test ./...
  downshift compact -- git status

Next: downshift context metrics, for the bytes saved so far
`,
	"version": `downshift version — Print the binary version (also --version)

Usage:
  downshift version

Flags:
  (none)

Examples:
  downshift version

Next: downshift doctor, to check hooks match this version
`,
	"help": `downshift help — Show the common commands, or every command with --all

Usage:
  downshift help [--all | <command>]

Flags:
  --all   Every command, grouped by who needs it

Examples:
  downshift help
  downshift help --all
  downshift help stats

Next: downshift setup, if hooks are not installed yet
`,
}
