// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"io"
)

// wantsAllHelp reports whether the arguments after "help" ask for the full
// command list.
func wantsAllHelp(rest []string) bool {
	for _, a := range rest {
		if a == "--all" || a == "-a" || a == "all" {
			return true
		}
	}
	return false
}

// usageShort is the default help: the few commands a user needs, in the order
// they use them. Everything else stays callable and is listed by
// "downshift help --all".
func usageShort(w io.Writer) {
	fmt.Fprint(w, `downshift — right-sized models for every subagent task

Get started
  downshift setup [--dry-run]      Wire the hooks into every harness found (backup, diff, --undo)
  downshift doctor                 Checklist: binary, hooks per harness, state, with a fix per failure
  downshift try "<task>"           What Downshift would pick for a task, without spawning anything

See the effect
  downshift stats                  Decisions and estimated savings (list price, not an invoice)
  downshift report                 Measured: routed vs control group, tokens per completed task, with CI
  downshift session                This session's subagents as a tree (--why, --pause)

Save context
  downshift compact -- <cmd>       Cut go test, git status, vitest output to its PASS/FAIL summary (exit code kept)

Fix things
  downshift config                 Every effective setting with its value and source

Hooks (your harness calls these, you do not)
  downshift claude-code | cursor | codex | antigravity | kirocrew | opencode

More
  downshift help --all             Every command: watch, models, quota, context, dci and the rest
  downshift version                Print the binary version

Examples
  downshift try "rename the variable userId"
  downshift try "rearchitect the payment flow" cursor claude-haiku-5-5
`)
}
