// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

// retrieveHelp covers the command that reads back output the compactor kept.
var retrieveHelp = map[string]string{
	"retrieve": `downshift retrieve — Print the full output the compactor kept for an id

Usage:
  downshift retrieve <id> [--grep RE] [--head N] [--tail N] [--lines A:B]

Flags:
  --grep RE     Keep only the lines matching this RE2 pattern
  --head N      Keep the first N lines
  --tail N      Keep the last N lines
  --lines A:B   Keep lines A to B (1-based, inclusive), each prefixed with its number

The id comes from the marker the compactor prints when DOWNSHIFT_CCR is on.
Kept output lives under the state directory for 24 hours. Exit 0 on success,
1 when the output is gone or expired, 2 on bad arguments.

Examples:
  downshift retrieve 3f9a1c0b7d2e
  downshift retrieve 3f9a1c0b7d2e --grep FAIL --head 20
  downshift retrieve 3f9a1c0b7d2e --lines 120:160

Next: downshift compact -- <command>, to keep and cut a command's output
`,
}
