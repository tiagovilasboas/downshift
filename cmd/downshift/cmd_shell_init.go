// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	_ "embed"
	"fmt"
	"io"
)

//go:embed shell_init.sh
var shellInitScript string

// runShellInit prints a shell script that sets up transparent compaction
// aliases. The canonical command is `downshift enable`; `shell-init` is kept
// as a legacy alias. Users source it in their shell rc or eval it directly:
//
//	eval "$(downshift enable)"
//
// The script only activates when stdout is not a terminal (agent environment),
// preserving full output for interactive debugging.
func runShellInit(args []string, out, errw io.Writer) int {
	for _, a := range args {
		if a == "-h" || a == "--help" {
			fmt.Fprint(out, shellInitUsage)
			return 0
		}
	}
	fmt.Fprint(out, shellInitScript)
	return 0
}

const shellInitUsage = `Usage: downshift enable

Print a shell script that sets up transparent compaction for agent environments.
Add to your shell configuration:

  # ~/.bashrc or ~/.zshrc
  eval "$(downshift enable)"

The script:
• Wraps allowlisted commands (go test, git status, npm test, etc.) through the compactor
• Runs any other command as typed (it asks the binary first: compactor --allowlisted)
• Only activates when stdout is not a terminal (agent context)
• Preserves full output for interactive debugging
• Respects DOWNSHIFT_SHELL_ALIASES=0 to disable

Environment variables:
  DOWNSHIFT_SHELL_ALIASES=0    Disable all aliases
  DOWNSHIFT_FORCE_COMPACT=1    Force compaction even in interactive terminals

Note: shell-init is kept as a legacy alias for this command.
`
