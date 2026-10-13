// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
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
  downshift doctor                 Check the install: version, state dir, catalog
  downshift try "<task>" [harness] See what Downshift would pick for a task

See the effect
  downshift stats [--days=N]       Decisions and estimated savings (default 30 days)
  downshift stats --export         Pasteable JSON summary, no prompts

Fix things
  downshift models list            The model catalog in effect
  downshift models discover        Ask each harness which models your account can use
  downshift models scores          Public capability scores: --refresh, --status, --quiet, --json
  downshift quota status           Budget evidence and freshness for a harness/model
  downshift feedback list          Routing ids waiting for your review

Hooks (your harness calls these, you do not)
  downshift claude-code | cursor | codex | antigravity | kirocrew

More
  downshift help --all             Every command, including maintainer tools
  downshift version                Print the binary version

Examples
  downshift try "rename the variable userId"
  downshift try "rearchitect the payment flow" cursor claude-haiku-5-5
`)
}
