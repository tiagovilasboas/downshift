// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/tiagovilasboas/downshift-ai/internal/console/reportview"
	"github.com/tiagovilasboas/downshift-ai/internal/console/shared/theme"
	"github.com/tiagovilasboas/downshift-ai/internal/console/surface"
)

// styledReportWanted is set once by main, from the raw argv (so --plain is
// still visible) and the real terminal. runReport reads it; tests leave it false.
var styledReportWanted bool

// wantsStyledReport says whether this invocation draws the styled report:
// `report` with nothing but --days=N, so `report clear` keeps its own path.
func wantsStyledReport(args []string) bool {
	if len(args) == 0 || args[0] != "report" {
		return false
	}
	for _, a := range args[1:] {
		if !strings.HasPrefix(a, "--days=") && a != "--plain" {
			return false
		}
	}
	return surface.Styled(launchFacts(args, os.Stdin, os.Stdout))
}

// reportProgram wraps the pure screen model as a Bubble Tea program on the
// alternate screen; it quits when the screen reports done.
type reportProgram struct{ m reportview.Model }

func (p reportProgram) Init() tea.Cmd { return p.m.Init() }

func (p reportProgram) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	p.m, cmd = p.m.Update(msg)
	if p.m.Done() {
		return p, tea.Quit
	}
	return p, cmd
}

func (p reportProgram) View() tea.View {
	v := tea.NewView(p.m.View())
	v.AltScreen = true
	return v
}

// showStyledReport runs the screen until the user closes it.
func showStyledReport(data reportview.Data) error {
	_, err := tea.NewProgram(reportProgram{m: reportview.New(theme.New(), data)}).Run()
	return err
}
