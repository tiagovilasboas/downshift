// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"io"
	"os"

	"github.com/tiagovilasboas/downshift/internal/catalog"
	"github.com/tiagovilasboas/downshift/internal/models"
	"github.com/tiagovilasboas/downshift/internal/paths"
	"github.com/tiagovilasboas/downshift/internal/setup"
)

// `downshift setup [harness...] [--dry-run] [--undo] [--yes]` wires every
// Downshift hook into the harness hook files listed in internal/setup/targets.go. It
// merges (never replaces), is idempotent, shows a diff and keeps a backup.
// The per-harness facts are data in setup.Targets; nothing here names a harness.

// runSetup is the CLI entry: it resolves HOME and the running binary.
func runSetup(args []string, in io.Reader, out, errw io.Writer) int {
	opts := defaultSetupOptions(in, out, errw)
	flags, help, bad := parseSetupArgs(args, &opts)
	if opts.Workers != "" && !setup.ValidWorkersMode(opts.Workers) && !help && bad == "" {
		fmt.Fprintf(errw, "setup: --workers %q is not valid: want off, read or write\n%s", opts.Workers, setupUsage)
		return 2
	}
	if help {
		fmt.Fprint(out, setupUsage)
		return 0
	}
	if bad != "" {
		fmt.Fprintf(errw, "setup: unknown flag %q\n%s", bad, setupUsage)
		return 2
	}
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintf(errw, "setup: cannot find the home directory: %v\n", err)
		return 1
	}
	opts.Home = home
	exe, err := os.Executable()
	if err != nil {
		fmt.Fprintf(errw, "setup: cannot find this binary: %v\n", err)
		return 1
	}
	opts.Binary = exe
	if code := setup.GuardHookBinary(opts, flags.allowUnsafe, os.TempDir()); code != 0 {
		return code
	}
	if !opts.Undo {
		reportStateMigration(opts)
	}
	if dir, err := paths.StateDir(); err == nil {
		opts.StateDir = dir
	}
	if cwd, err := os.Getwd(); err == nil && !flags.noPolicy {
		opts.ProjectDir = cwd
		if md, ok := any(catalog.Load()).(models.Metadata); ok {
			opts.SmallTargets = resolveSmallTargets(md)
		}
	}
	if !flags.keepBinaries {
		opts.BinDirs = setup.DefaultBinDirs(home)
	}
	if !opts.DryRun && !opts.Undo {
		// Older releases created the state directory and its files with the
		// default umask; tighten what the product owns. Best effort.
		_ = paths.Harden()
	}
	if wantsSetupWizard(opts, isInteractiveTerminal(opts.In)) {
		return setupTUI(opts)
	}
	return setupRun(opts)
}

// setupRun runs setup.Run with the pieces that live outside internal/setup:
// this build's stamp and the model discovery and session-model report, which
// need internal/core. A hook the caller already set is kept.
func setupRun(o setup.Options) int {
	if o.BuildDate == "" {
		o.BuildDate = buildDate
	}
	if o.DiscoverModels == nil {
		o.DiscoverModels = setupDiscoverModels
	}
	if o.EnsureSessionModels == nil {
		o.EnsureSessionModels = setupEnsureSessionModels
	}
	return setup.Run(o)
}

// reportStateMigration moves legacy state into the new state directory (setup
// only) and tells the user what happened; a dry run only reports.
func reportStateMigration(o setup.Options) {
	m, err := paths.MigrateLegacy(o.DryRun)
	if err != nil {
		fmt.Fprintf(o.Err, "setup: state migration failed: %v\n", err)
		return
	}
	fmt.Fprint(o.Out, m.Describe())
}

// wantsSetupWizard is the default entry: a person in a terminal gets the
// wizard, with or without harness names. Only the machine flags (--yes,
// --dry-run, --undo) and --no-tui keep the plain flow.
func wantsSetupWizard(o setup.Options, tty bool) bool {
	return (o.TUI || tty) && !o.NoTUI && !o.Yes && !o.DryRun && !o.Undo
}

const setupUsage = `usage: downshift setup [harness...] [--dry-run] [--undo] [--yes] [--tui] [--no-tui] [--no-session-policy] [--you-should-know] [--no-plugin] [--no-mods] [--no-mcp] [--no-skills] [--no-steering] [--workers off|read|write] [--no-policy] [--keep-binaries] [--allow-unsafe-binary]

Wire every Downshift hook into the harnesses found on this machine.
  --tui                run interactive terminal UI onboarding
  --no-tui             disable interactive onboarding even in a terminal
  --dry-run            show the diff, write nothing
  --undo               restore the latest backup of each hook file
  --yes                do not ask before writing
  --no-session-policy  wire the hooks only: do not set the Claude Code
                       main-session policy (model opusplan, sessions start in
                       plan mode)
  --you-should-know    also turn on Claude Code's built-in You should know mod
                       (opt-in: each check is a billed background request)
  --no-plugin          Claude Code: keep the hooks in settings.json instead of
                       installing the Downshift plugin (the plugin carries the
                       same hooks)
  --no-mods            Claude Code: do not install the downshift-agents mod
                       (live band and pane for subagents; it only observes) in
                       ~/.claude/skills, nor remove it from CLAUDE_CODE_PLUGIN_DIRS
                       in a shell profile
  --no-mcp            do not register "downshift mcp serve" as the MCP server
                       "downshift" in Claude Code (through its CLI) and Codex
                       (a marked block in config.toml). Registered by default;
                       --undo removes it. --mcp is still accepted
  --no-skills          do not install the Downshift skills
  --no-steering        do not write the instructions file (steering and agent
                       triggers)
  --workers MODE       set orchestration.workers in ~/.downshift/policy.json:
                       off, read or write (write lets workers edit files in
                       their own git worktree; never merged). Without it,
                       --yes writes nothing about workers and the wizard asks
  --no-policy          do not offer .downshift/policy.json in the current
                       directory (see downshift policy init)
  --keep-binaries      do not offer to remove other downshift copies on PATH
                       and in the Go and per-user bin directories
  --allow-unsafe-binary
                       write hooks that point at this binary even when it sits
                       in a temporary, Go build or world-writable directory

Claude Code gets the policy by default, only for keys it does not already set.
`
