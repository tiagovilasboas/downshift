// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/tiagovilasboas/downshift/internal/catalog"
	"github.com/tiagovilasboas/downshift/internal/paths"
	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

// isConfigured reports whether Downshift is already set up on the host machine.
// It checks whether the state directory has events or policy, or whether any
// supported harness already has Downshift hooks wired into its config file.
func isConfigured(home string) bool {
	if home == "" {
		h, err := os.UserHomeDir()
		if err != nil {
			return false
		}
		home = h
	}
	// Check if event log exists and has data.
	if evPath, err := paths.EventsPath(); err == nil {
		if fi, err := os.Stat(evPath); err == nil && fi.Size() > 0 {
			return true
		}
	}
	// Check if state dir has policy or session models.
	if stateDir, err := paths.StateDir(); err == nil {
		for _, f := range []string{"policy.json", "session-models.json", "config.json"} {
			if fi, err := os.Stat(filepath.Join(stateDir, f)); err == nil && fi.Size() > 0 {
				return true
			}
		}
	}
	// Check if any installed harness hook file references downshift.
	for _, target := range setupTargets {
		if target.File == "" {
			continue
		}
		full := filepath.Join(home, target.File)
		data, err := os.ReadFile(full)
		if err != nil {
			continue
		}
		if strings.Contains(string(data), "downshift") {
			return true
		}
	}
	return false
}

// runDefaultEntry is invoked when `downshift` is run with zero arguments.
// If the developer has not configured Downshift yet, it automatically runs the
// zero-prompt setup in 1 second. If already configured, it presents a scannable
// dashboard of savings and session status.
func runDefaultEntry(in io.Reader, out, errw io.Writer, cat *catalog.Catalog) int {
	home, _ := os.UserHomeDir()
	if !isConfigured(home) {
		fmt.Fprintln(out, "Downshift: first run detected. Setting up hooks automatically (1s)...")
		code := runSetup([]string{"--yes"}, in, out, errw)
		if code == 0 {
			fmt.Fprintln(out, "\n✓ Downshift is now active! Subagent routing and terminal compaction are running.")
			fmt.Fprintln(out, "Run `downshift` anytime to inspect your token savings and session health.")
		}
		return code
	}

	fmt.Fprintf(out, "Downshift %s · Deterministic Token Economizer\n", buildVersion)
	fmt.Fprintln(out, strings.Repeat("─", 50))

	// Display session savings if available.
	sessionCode := runSession([]string{"--brief"}, out, errw, cat)
	if sessionCode != 0 {
		// Fallback to stats if session panel is quiet.
		events, err := telemetry.ReadEvents()
		if err == nil && len(events) > 0 {
			telemetry.PrintStats(events, telemetry.StatsOptions{Days: 30}, out)
		} else {
			fmt.Fprintln(out, "No subagent spawns recorded yet in this session.")
			fmt.Fprintln(out, "Start a coding task in Claude Code, Cursor, or Codex to begin saving tokens.")
		}
	}

	return 0
}
