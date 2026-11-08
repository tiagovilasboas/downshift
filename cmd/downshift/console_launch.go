// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"io"
	"os"
	"slices"

	"github.com/charmbracelet/x/term"
	"github.com/tiagovilasboas/downshift-ai/internal/console"
	"github.com/tiagovilasboas/downshift-ai/internal/console/surface"
)

// stripPlain removes the human --plain escape hatch so handlers never see it.
func stripPlain(args []string) []string {
	return slices.DeleteFunc(slices.Clone(args), func(a string) bool { return a == "--plain" })
}

// launchFacts collects the terminal facts for args.
func launchFacts(args []string, in, out *os.File) surface.Facts {
	return surface.Facts{
		Args:     args,
		StdinTTY: term.IsTerminal(in.Fd()), StdoutTTY: term.IsTerminal(out.Fd()),
		Getenv: os.Getenv,
	}
}

// consoleLaunch opens the console on args when a person runs a read-only
// command in a terminal. handled=false means the normal plain path runs.
func consoleLaunch(args []string) (code int, handled bool) {
	if !surface.Decide(launchFacts(args, os.Stdin, os.Stdout), consoleCommands).Console {
		return 0, false
	}
	exe, err := os.Executable()
	if err != nil {
		return 0, false
	}
	return launchConsole(args, exe, os.Stdin, os.Stdout, os.Stderr)
}

// launchConsole runs the console started on args, then flushes its last
// output. A console that fails before any run falls back (handled=false).
func launchConsole(args []string, exe string, in io.Reader, out, errw io.Writer) (int, bool) {
	oc, err := console.Run(console.Options{
		Exe: exe, Version: "downshift " + buildVersion,
		Commands: consoleCommands, In: in, Out: out, Start: args,
	})
	if err != nil {
		return 0, false
	}
	return flushOutcome(oc, out, errw), true
}

// flushOutcome writes the captured streams and returns the child's code.
func flushOutcome(oc console.Outcome, out, errw io.Writer) int {
	fmt.Fprint(out, oc.Stdout)
	fmt.Fprint(errw, oc.Stderr)
	return oc.Code
}
