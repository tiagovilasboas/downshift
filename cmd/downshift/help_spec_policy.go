// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

// policyHelp covers the routing policy file check.
var policyHelp = map[string]string{
	"policy": `downshift policy — Validate the routing policy without routing anything

Usage:
  downshift policy check
  downshift policy init [--dry-run] [--yes]

Reads the nearest project .downshift/policy.json (searched upward from the
current directory) and the user's policy.json in the state directory, and
checks the routing section of each. The tier floor in the same files is
checked too. Exits 0 when both are usable; exits 1 with the first refusal
(file, field and problem) otherwise. A missing file is fine: no profiles and
no rules mean the requested model is kept.

Flags:
  none

policy init creates .downshift/policy.json in the current directory with no
routing policy. It asks before writing; --dry-run shows the file and writes
nothing; --yes skips the question. An existing file is left untouched.
downshift setup offers the same file, unless --no-policy is passed.

Examples:
  downshift policy check
  downshift policy init --dry-run

Next: downshift config, to see every effective setting and where it comes from
`,
}
