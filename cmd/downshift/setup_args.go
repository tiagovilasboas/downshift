// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"io"
	"strings"
	"time"

	"github.com/tiagovilasboas/downshift-ai/internal/setup"
)

// setupArgFlags are the switches the CLI resolves itself: the hook binary
// guard, the policy offer and the binary cleanup. The rest land in setup.Options.
type setupArgFlags struct {
	allowUnsafe  bool
	noPolicy     bool
	keepBinaries bool
}

// defaultSetupOptions is what `downshift setup` runs with before any flag.
// The MCP registration is on by default; --no-mcp turns it off.
func defaultSetupOptions(in io.Reader, out, errw io.Writer) setup.Options {
	return setup.Options{In: in, Out: out, Err: errw, Now: time.Now, Exec: setup.RealExec(), Mcp: true}
}

// parseSetupArgs sets the flags on opts and returns the switches setup resolves
// itself. help is true for -h or --help; bad is the first unknown flag.
func parseSetupArgs(args []string, opts *setup.Options) (flags setupArgFlags, help bool, bad string) {
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--workers":
			if i+1 < len(args) {
				i++
				opts.Workers = args[i]
			} else {
				opts.Workers = "(missing)"
			}
		case strings.HasPrefix(a, "--workers="):
			opts.Workers = strings.TrimPrefix(a, "--workers=")
			if opts.Workers == "" {
				opts.Workers = "(empty)"
			}
		case a == "--dry-run":
			opts.DryRun = true
		case a == "--undo":
			opts.Undo = true
		case a == "--tui":
			opts.TUI = true
		case a == "--no-tui":
			opts.NoTUI = true
		case a == "--yes" || a == "-y":
			opts.Yes = true
		case a == "--no-session-policy":
			opts.NoSessionPolicy = true
		case a == "--you-should-know":
			opts.YouShouldKnow = true
		case a == "--no-plugin":
			opts.NoPlugin = true
		case a == "--no-mods":
			opts.NoMods = true
		case a == "--mcp":
			opts.Mcp = true
		case a == "--no-mcp":
			opts.Mcp = false
		case a == "--no-skills":
			opts.NoSkills = true
		case a == "--no-steering":
			opts.NoSteering = true
		case a == "--no-policy":
			flags.noPolicy = true
		case a == "--allow-unsafe-binary":
			flags.allowUnsafe = true
		case a == "--keep-binaries":
			flags.keepBinaries = true
		case a == "-h" || a == "--help":
			return flags, true, ""
		case strings.HasPrefix(a, "-"):
			return flags, false, a
		default:
			opts.Names = append(opts.Names, a)
		}
	}
	return flags, false, ""
}
