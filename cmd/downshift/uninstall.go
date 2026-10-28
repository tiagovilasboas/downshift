// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/tiagovilasboas/downshift/internal/paths"
)

// runUninstall is `downshift uninstall [--yes] [--dry-run] [--purge]`:
// Restores all harness hook configurations from their latest backups (.bak)
// and cleans up generated wrapper scripts. With --purge, also cleans state caches.
func runUninstall(args []string, in io.Reader, out, errw io.Writer) int {
	purge := false
	var setupArgs []string
	setupArgs = append(setupArgs, "--undo")

	for _, a := range args {
		switch a {
		case "--purge":
			purge = true
		case "-h", "--help":
			fmt.Fprint(out, uninstallUsage)
			return 0
		default:
			setupArgs = append(setupArgs, a)
		}
	}

	fmt.Fprintln(out, "Downshift: uninstalling hooks and restoring harness configs...")
	rc := runSetup(setupArgs, in, out, errw)
	if rc != 0 {
		return rc
	}

	dryRun := false
	for _, a := range args {
		if a == "--dry-run" {
			dryRun = true
			break
		}
	}

	if !dryRun {
		if stateDir, err := paths.StateDir(); err == nil {
			binDir := filepath.Join(stateDir, "bin")
			_ = os.RemoveAll(binDir)
			if purge {
				cacheDir := filepath.Join(stateDir, "cache")
				_ = os.RemoveAll(cacheDir)
				fmt.Fprintln(out, "Downshift: caches purged.")
			}
		}
		fmt.Fprintln(out, "\n✓ Downshift uninstalled. All harness hooks restored to previous state.")
	} else {
		fmt.Fprintln(out, "\nDry run: no changes written.")
	}

	return 0
}

const uninstallUsage = `usage: downshift uninstall [--dry-run] [--yes] [--purge]

Restore every harness hook configuration from its latest backup and remove wrappers.
  --dry-run   Show what would be restored, write nothing
  --yes, -y   Do not prompt before restoring
  --purge     Also remove local cache directories in ~/.downshift
`
