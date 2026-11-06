// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

// Package console is Downshift's full-screen terminal UI. It owns no command
// logic: every number it shows comes from running a Downshift command as a
// child process of the same binary, so the console and the CLI never disagree.
//
// Screens live one per directory (home, palette, output); shared holds the
// theme, the frame and the command runner.
//
// Downshift by Tiago Vilas Boas
// https://github.com/tiagovilasboas/downshift
package console

import (
	"io"

	tea "charm.land/bubbletea/v2"
	"github.com/tiagovilasboas/downshift/internal/console/shared/command"
)

// Options configures one console run.
type Options struct {
	// Exe is the Downshift binary the console runs commands with.
	Exe string
	// Version is shown on the bottom line.
	Version string
	// Commands is every command the palette lists.
	Commands []command.Spec
	// Start, when set, opens the console on that command's output instead of
	// the home screen; esc or q then quits rather than going back.
	Start []string
	In    io.Reader
	Out   io.Writer
}

// Outcome is the last captured run, for the caller to print once the alternate
// screen has closed. It is the zero value when no command ran.
type Outcome struct {
	Stdout, Stderr string
	Code           int
}

// Run shows the console until the user quits.
func Run(o Options) (Outcome, error) {
	m, err := tea.NewProgram(newApp(o), tea.WithInput(o.In), tea.WithOutput(o.Out)).Run()
	a, ok := m.(app)
	if !ok {
		return Outcome{}, err
	}
	return Outcome{Stdout: a.last.Stdout, Stderr: a.last.Stderr, Code: a.last.Code}, err
}
