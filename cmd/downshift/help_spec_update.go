// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

var updateHelp = map[string]string{
	"update": `downshift update — Install the latest release, then refresh every harness

Usage:
  downshift update [--check] [--yes] [--version vX.Y.Z]

Flags:
  --check            Say whether a newer release exists; change nothing
  --yes, -y          Pass --yes to setup: write the harness files without asking
  --version vX.Y.Z   Install this release instead of the latest

Steps: download the archive for this platform, check its sha256 against
checksums.txt (and build provenance when the gh CLI is installed), replace
this binary in place, then run the new binary's setup (diff, then a question)
and doctor. A current binary skips the download; setup still runs.

Examples:
  downshift update --check
  downshift update
  downshift update --version v0.14.0

Next: open a new harness session so it loads the updated hooks
`,
}
