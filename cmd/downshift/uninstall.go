// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

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
		home, _ := os.UserHomeDir()
		if stateDir, err := paths.StateDir(); err == nil {
			if purge {
				_ = os.RemoveAll(stateDir)
				purgeBackups(home)
				fmt.Fprintln(out, "Downshift: all state files, caches, and backups purged.")
			} else {
				binDir := filepath.Join(stateDir, "bin")
				_ = os.RemoveAll(binDir)
			}
		}
		fmt.Fprintln(out, "\n✓ Downshift uninstalled. All harness hooks restored to previous state.")
	} else {
		fmt.Fprintln(out, "\nDry run: no changes written.")
	}

	return 0
}

func purgeBackups(home string) {
	if home == "" {
		return
	}
	harnessDirs := []string{
		filepath.Join(home, ".claude"),
		filepath.Join(home, ".codex"),
		filepath.Join(home, ".cursor"),
		filepath.Join(home, ".kiro"),
		filepath.Join(home, ".gemini", "config"),
		filepath.Join(home, ".config", "opencode"),
	}
	for _, dir := range harnessDirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if !e.IsDir() && strings.Contains(e.Name(), setupBackupInfix) {
				_ = os.Remove(filepath.Join(dir, e.Name()))
			}
		}
	}
}

const uninstallUsage = `usage: downshift uninstall [--dry-run] [--yes] [--purge]

Restore every harness hook configuration from its latest backup and remove wrappers.
  --dry-run   Show what would be restored, write nothing
  --yes, -y   Do not prompt before restoring
  --purge     Also remove ~/.downshift state dir and purge all backup files
`
