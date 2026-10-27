// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

// maintainerHelp covers the commands for people who train and evaluate the
// classifier. They need labelled data that ships outside this repository.
var maintainerHelp = map[string]string{
	"eval-outcome": `downshift eval-outcome — Outcome eval: executable checks, small vs frontier pass rate

Usage:
  downshift eval-outcome --verify
  downshift eval-outcome --record --tier=small|mid|frontier --model=ID --solver='<cmd>'
  downshift eval-outcome --report [--write=FILE | --check=FILE]

Flags:
  --verify          Offline: every stub fails, every reference passes, recorded results reproduce
  --record          Solve every task with --solver (makes model calls; refused in CI)
  --tier=TIER       Tier label for --record: small, mid or frontier
  --model=ID        Model id recorded with --record
  --solver='CMD'    Shell command for --record: prompt on stdin, answer on stdout
  --report          Print the generated README block
  --write=FILE      With --report: rewrite the marked block in FILE
  --check=FILE      With --report: exit 1 if the marked block in FILE is stale
  --tasks=DIR       Task directory
  --runs=DIR        Recorded runs directory

--verify runs the task and solution Go code with your privileges and no sandbox.

Examples:
  downshift eval-outcome --verify
  downshift eval-outcome --report --check=README.md

Next: downshift models check, to see new or untiered models
`,
}
