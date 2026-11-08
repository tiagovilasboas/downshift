// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"io"
	"os"

	"github.com/charmbracelet/x/term"
	"github.com/tiagovilasboas/downshift-ai/internal/console"
	"github.com/tiagovilasboas/downshift-ai/internal/console/surface"
)

// wantsConsole reports whether a bare `downshift` should open the console. The
// decision is shared (surface.Decide); this only collects the terminal facts.
func wantsConsole(in, out *os.File) bool {
	return surface.Decide(surface.Facts{
		StdinTTY: term.IsTerminal(in.Fd()), StdoutTTY: term.IsTerminal(out.Fd()),
		Getenv: os.Getenv,
	}, consoleCommands).Console
}

// runConsole opens the full-screen console. Any failure to start it falls back
// to the plain entry's caller with a non-zero code and a reason.
func runConsole(in io.Reader, out, errw io.Writer) int {
	exe, err := os.Executable()
	if err != nil {
		fmt.Fprintf(errw, "downshift: cannot find own binary for the console: %v\n", err)
		return 1
	}
	_, err = console.Run(console.Options{
		Exe: exe, Version: "downshift " + buildVersion,
		Commands: consoleCommands, In: in, Out: out,
	})
	if err != nil {
		fmt.Fprintf(errw, "downshift: console: %v\n", err)
		return 1
	}
	return 0
}
