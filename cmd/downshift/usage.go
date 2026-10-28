// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"io"
)

func usage(w io.Writer) {
	fmt.Fprint(w, `downshift — right-sized models for every subagent task

Everyday
  downshift setup [--dry-run]    Wire the hooks into every harness found (backup, diff, --undo)
  downshift dashboard [--port=1337]  Open the Downshift dashboard in browser (sessions, agents, tokens)
  downshift enable               Print shell aliases for transparent compaction (eval in .bashrc/.zshrc)
  downshift shell-init           Deprecated alias of enable; kept for old shell rc files
  downshift compact -- <cmd>     Cut a test or build command's output to its PASS/FAIL summary
  downshift doctor [--verbose] [--json]  Checklist with a fix command per failure
  downshift stats [--days=N]     Show routing decisions and estimated savings (default: 30 days)
  downshift report [--days=N] [--json]   Routed vs control tokens per completed task, rework, savings with 95% CI
  downshift session [--why|--pause|--resume] [--json]  This session's subagents as a tree (every harness)
  downshift config [--json]      Every effective setting with its value and source

More about what happened
  downshift stats --detail       Full report: units, honor, quota, usage, per-session tokens
  downshift stats --dci          Code Intelligence (DCI) telemetry: injections, abstentions, latencies
  downshift stats --export       Print a pasteable JSON summary without prompts
  downshift watch [--session=<id>] [--interval=1s]      Live session tree, redrawn every interval (Ctrl-C to quit)

Models and quota
  downshift models list          Show the effective catalog (embedded or override)
  downshift models available --harness ID   The models a spawn can use now on that harness, and where each came from
  downshift route --harness ID [--model ID] [--tier T] [--json]   Show what would be applied for one spawn
  downshift playbook list|show|edit|add|remove|restore|check   The working rules a delegated worker receives
  downshift models discover      Ask each harness which models the account can use; hooks read the result
  downshift quota status         Inspect budget evidence and freshness for a harness/model

Plumbing (your harness or statusline calls these, you do not)
  downshift claude-code          Run as a Claude Code PreToolUse hook (reads stdin)
  downshift cursor               Run as a Cursor preToolUse hook (reads stdin)
  downshift codex                Run as a Codex PreToolUse hook (reads stdin)
  downshift antigravity          Run as an Antigravity PreToolUse hook
  downshift kirocrew             Run as a KiroCrew preToolUse hook (policy mode: exit 0/2)
  downshift opencode             Run as the OpenCode subagent_spawn_intent route hook (wire v1)
  downshift quota import         Import native subscription quota JSON from stdin
  downshift quota collect        Collect quota from an explicit local Codex transcript
  downshift quota claude-statusline   Opt-in Claude statusLine quota bridge

Maintainer (needs labelled data, which lives in downshift-labs)
  downshift eval-outcome --verify|--report  Outcome eval: executable checks, small vs frontier pass rate
  downshift probe <event>        As a hook: record a payload's field names (never prompts or paths) in probe.jsonl; see docs/guides/probe.md
  downshift models check         Query provider APIs and report new/untiered models
  downshift models pull          Write user catalog.json under the state dir

Experimental (kept, not part of the routing pitch; may change or move)
  downshift context status       Show context optimization status and active provider
  downshift context providers    List supported context optimization companion providers
  downshift context doctor       Run diagnostics on context optimization providers
  downshift context enable [p]   Enable context optimization provider (default: native; --exec adds the shell wrapper)
  downshift context disable      Disable context optimization and revert configs
  downshift context metrics      Show token reduction metrics from active provider
  downshift context compress     Compress tool output from stdin (observe/safe)
  downshift context benchmark    Compare 4 optimization scenarios (routing vs compression)
  downshift dci status           Show DCI status, mode, and index statistics
  downshift dci enable [act|obs] Enable DCI code intelligence (active: inject; observe: telemetry)
  downshift dci disable          Disable DCI (mode off)
  downshift dci doctor           Diagnose DCI health, file limits, permissions, and budgets
  downshift dci inspect <prompt> Simulate retrieval and context generation for a query
  downshift dci benchmark        Run automated latency, relevance, and budget benchmarks
  go build -o dsmon ./cmd/dsmon  Live terminal widget (separate binary; downshift watch is the supported live view)
  ./cmd/dsmon/launch.sh          Open dsmon in a floating terminal window

  downshift version              Print the binary version (also --version)

Grok note:
  Grok routes subagent models via config, not a hook (its PreToolUse is
  allow/deny only). Pin the reasoning effort on a subagent role in
  ~/.grok/config.toml. See the README.

Examples:
  downshift stats
  downshift stats --days=7
`)
}
