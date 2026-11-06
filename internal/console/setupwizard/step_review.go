// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package setupwizard

import (
	tea "charm.land/bubbletea/v2"
	"github.com/tiagovilasboas/downshift/internal/console/shared/diffview"
	"github.com/tiagovilasboas/downshift/internal/console/shared/frame"
	"github.com/tiagovilasboas/downshift/internal/console/shared/theme"
)

// startReview moves to the review step and computes the diff as a command.
func (m Model) startReview() (Model, tea.Cmd) {
	m.step = stepReview
	m.loading, m.err, m.files = true, nil, nil
	m.diff = diffview.New(m.th, nil)
	m.gen++
	cmds := []tea.Cmd{m.previewCmd()}
	if !m.ticking {
		m.ticking = true
		cmds = append(cmds, frame.Tick())
	}
	return m, tea.Batch(cmds...)
}

func (m Model) previewCmd() tea.Cmd {
	gen, sel, preview := m.gen, m.Selection(), m.o.Preview
	return func() tea.Msg {
		if preview == nil {
			return previewMsg{gen: gen}
		}
		files, err := preview(sel)
		return previewMsg{gen: gen, files: files, err: err}
	}
}

func (m Model) previewed(p previewMsg) Model {
	if p.gen != m.gen || m.step != stepReview {
		return m
	}
	m.loading, m.err, m.files = false, p.err, p.files
	m.diff = diffview.New(m.th, p.files)
	m.diff, _ = m.diff.Update(tea.WindowSizeMsg{Width: m.width, Height: m.height - 1})
	return m
}

// leaveReview moves to the confirmation once a diff (or an empty result) is shown.
func (m Model) leaveReview() Model {
	if m.loading || m.err != nil {
		return m
	}
	m.confirm = newConfirm(m)
	m.confirm, _ = m.confirm.Update(tea.WindowSizeMsg{Width: m.width, Height: m.height})
	m.step = stepConfirm
	return m
}

func (m Model) viewReview() string {
	back := []frame.Key{{Key: "esc", Help: "back"}}
	switch {
	case m.loading:
		return m.screen("  "+m.th.Accent.Render(frame.Spinner(m.tick))+" computing the dry-run diff…", back)
	case m.err != nil:
		return m.screen("  "+m.th.Fail.Render(theme.SymFail+" preview failed: "+m.err.Error()), back)
	case len(m.files) == 0:
		return m.screen("  "+m.th.Muted.Render("Nothing to change."),
			[]frame.Key{{Key: "enter", Help: "next"}, {Key: "esc", Help: "back"}})
	}
	return m.header() + "\n" + m.diff.View()
}
