// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
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
  downshift doctor [--verbose] [--json]  Checklist with a fix command per failure
  downshift try "<task>" [harness] [model]   Test classification from the terminal
  downshift stats [--days=N]     Show routing decisions and estimated savings (default: 30 days)
  downshift report [--days=N] [--json]   Routed vs control cost per completed task, rework, savings with 95% CI
  downshift session [--why|--pause|--resume] [--json]  This session's subagents as a tree (every harness)
  downshift config [--json]      Every effective setting with its value and source
  downshift feedback list        List routing IDs awaiting engineer review

More about what happened
  downshift stats --detail       Full report: units, honor, quota, usage, per-session cost
  downshift stats --export       Print a pasteable JSON summary without prompts
  downshift stats --cost-per-unit=<USD>   Convert normalised units to dollars
  downshift watch [--session=<id>] [--interval=1s]      Live session tree, redrawn every interval (Ctrl-C to quit)
  downshift feedback stats       Summarize outcomes per harness
  downshift feedback <id> <outcome>  Record success, retry, or failed

Models and quota
  downshift models list          Show the effective catalog (embedded or override)
  downshift models discover      Ask each harness which models the account can use; hooks read the result
  downshift models scores        Public capability scores cache (--refresh, --status, --quiet, --json)
  downshift quota status         Inspect budget evidence and freshness for a harness/model

Plumbing (your harness or statusline calls these, you do not)
  downshift claude-code          Run as a Claude Code PreToolUse hook (reads stdin)
  downshift cursor               Run as a Cursor preToolUse hook (reads stdin)
  downshift codex                Run as a Codex PreToolUse hook (reads stdin)
  downshift antigravity          Run as an Antigravity PreToolUse hook
  downshift kirocrew             Run as a KiroCrew preToolUse hook (policy mode: exit 0/2)
  downshift quota import         Import native subscription quota JSON from stdin
  downshift quota collect        Collect quota from an explicit local Codex transcript
  downshift quota claude-statusline   Opt-in Claude statusLine quota bridge

Maintainer (needs labelled data, which lives in downshift-labs)
  downshift benchmark <file>     Run classifier against a labelled dataset; print confusion matrix
  downshift benchmark <file> --compare  Compare Legacy vs CapabilityRouter v2 side by side
  downshift eval-outcome --verify|--report  Outcome eval: executable checks, small vs frontier pass rate
  downshift eval-adapt --runs=<dir>  Offline adapt memory efficacy from recorded tier runs (maintainer)
  downshift train <file>         Train capability-router v2 on a labelled dataset
  downshift train --from-events  Train from engineer-reviewed local feedback
  downshift shadow-report        Candidate observations vs reviewed labels, plus E5 shadow/fuse summary (JSON; --text)
  downshift probe <event>        As a hook: record a payload's field names (never prompts or paths) in probe.jsonl; see docs/guides/probe.md
  downshift models check         Query provider APIs and report new/untiered models
  downshift models pull          Write user catalog.json under the state dir

Experimental (kept, not part of the routing pitch; may change or move)
  downshift serve                Start the web dashboard at http://localhost:7474 (serves web/)
  downshift context status       Show context optimization status and active provider
  downshift context providers    List supported context optimization companion providers
  downshift context doctor       Run diagnostics on context optimization providers
  downshift context enable [p]   Enable context optimization provider (default: native)
  downshift context disable      Disable context optimization and revert configs
  downshift context metrics      Show token reduction metrics from active provider
  downshift context compress     Compress tool output from stdin (observe/safe)
  downshift context benchmark    Compare 4 optimization scenarios (routing vs compression)
  go build -o dsmon ./cmd/dsmon  Live terminal widget (separate binary; downshift watch is the supported live view)
  ./cmd/dsmon/launch.sh          Open dsmon in a floating terminal window

  downshift version              Print the binary version (also --version)

Grok note:
  Grok routes subagent models via config, not a hook (its PreToolUse is
  allow/deny only). Run  downshift try "<task>" grok  to see the reasoning
  effort to pin on a subagent role in ~/.grok/config.toml. See the README.

Examples:
  downshift try "rename the variable userId"
  downshift try "rearchitect the payment flow" cursor claude-haiku-5-5
  downshift try "add a subagent to scan for secrets" codex gpt-5.6-sol
  downshift try "explore the auth module" grok
  downshift stats
  downshift stats --days=7
  downshift benchmark path/to/tasks.json
`)
}
