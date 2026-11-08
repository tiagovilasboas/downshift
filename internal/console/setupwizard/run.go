// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package setupwizard

import (
	"io"

	tea "charm.land/bubbletea/v2"
	"github.com/tiagovilasboas/downshift-ai/internal/console/shared/theme"
)

// program adapts Model to tea.Model and quits once the wizard is over.
type program struct{ m Model }

func (p program) Init() tea.Cmd { return nil }

func (p program) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	p.m, cmd = p.m.Update(msg)
	if p.m.Done() || p.m.Cancelled() {
		return p, tea.Quit
	}
	return p, cmd
}

func (p program) View() tea.View {
	v := tea.NewView(p.m.View())
	v.AltScreen = true
	return v
}

// Run shows the wizard until the user confirms or leaves. confirmed is true
// only after the final yes; the selection is the choice at that point.
func Run(o Options, in io.Reader, out io.Writer) (sel Selection, confirmed bool, err error) {
	final, err := tea.NewProgram(program{New(theme.New(), o)}, tea.WithInput(in), tea.WithOutput(out)).Run()
	p, ok := final.(program)
	if err != nil || !ok {
		return Selection{}, false, err
	}
	return p.m.Selection(), p.m.Done(), nil
}
