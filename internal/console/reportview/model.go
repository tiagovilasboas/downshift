// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package reportview

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/tiagovilasboas/downshift/internal/console/shared/frame"
	"github.com/tiagovilasboas/downshift/internal/console/shared/theme"
)

const (
	defaultWidth  = 80
	defaultHeight = 24
	chrome        = 3 // header, tab bar and footer lines
)

// Model is the screen state. Tab 0 is the summary; the rest are Data.Tabs.
type Model struct {
	th            theme.Theme
	data          Data
	tab, top      int
	width, height int
	done          bool
}

// New builds the screen over data.
func New(th theme.Theme, data Data) Model {
	return Model{th: th, data: data, width: defaultWidth, height: defaultHeight}
}

// Done reports whether the user closed the screen.
func (m Model) Done() bool { return m.done }

// Init has nothing to start.
func (m Model) Init() tea.Cmd { return nil }

func (m Model) tabCount() int { return 1 + len(m.data.Tabs) }

func (m Model) bodyHeight() int { return max(1, m.height-chrome) }

func (m Model) body() []string {
	if m.tab == 0 {
		return m.summaryLines()
	}
	return m.data.Tabs[m.tab-1].Lines
}

func (m Model) maxTop() int { return max(0, len(m.body())-m.bodyHeight()) }

func (m *Model) scroll(to int) { m.top = min(max(to, 0), m.maxTop()) }

func (m *Model) goTab(i int) {
	n := m.tabCount()
	m.tab = ((i % n) + n) % n
	m.top = 0
}

// Update handles keys and window size.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.scroll(m.top)
	case tea.KeyPressMsg:
		m.key(msg.String())
	}
	return m, nil
}

func (m *Model) key(k string) {
	page := m.bodyHeight()
	switch k {
	case "right", "l", "tab":
		m.goTab(m.tab + 1)
	case "left", "h", "shift+tab":
		m.goTab(m.tab - 1)
	case "1", "2", "3", "4", "5":
		if i := int(k[0] - '1'); i < m.tabCount() {
			m.goTab(i)
		}
	case "up", "k":
		m.scroll(m.top - 1)
	case "down", "j":
		m.scroll(m.top + 1)
	case "pgup":
		m.scroll(m.top - page)
	case "pgdown":
		m.scroll(m.top + page)
	case "g", "home":
		m.scroll(0)
	case "G", "end":
		m.scroll(m.maxTop())
	case "q", "esc":
		m.done = true
	}
}

// View draws the headline, tab bar, body and key footer.
func (m Model) View() string {
	lines := m.body()
	end := min(m.top+m.bodyHeight(), len(lines))
	title, step := m.summaryHeader(), ""
	if m.tab > 0 {
		title = m.data.Tabs[m.tab-1].Title
		step = fmt.Sprintf("%d–%d of %d", min(m.top+1, len(lines)), end, len(lines))
	}
	head := frame.Header(title, step, m.width)
	if !m.th.NoColor {
		head = m.th.Bold.Render(head)
	}
	rows := []string{head, m.tabBar()}
	for _, l := range lines[m.top:end] {
		rows = append(rows, m.rowLine(l))
	}
	for len(rows) < m.bodyHeight()+2 {
		rows = append(rows, "")
	}
	keys := []frame.Key{{Key: "←/→", Help: "tab"}, {Key: "1-5", Help: "jump"}, {Key: "q", Help: "quit"}}
	if m.tab > 0 {
		keys = []frame.Key{{Key: "←/→", Help: "tab"}, {Key: "↑/↓", Help: "scroll"}, {Key: "g/G", Help: "top/bottom"}, {Key: "q", Help: "quit"}}
	}
	rows = append(rows, m.th.Muted.Render(frame.Footer(keys, m.width)))
	return strings.Join(rows, "\n")
}

// rowLine clips a body line. Summary lines are styled and already fit; detail
// lines are plain.
func (m Model) rowLine(l string) string {
	if m.tab == 0 {
		return l
	}
	return frame.Clip(l, m.width)
}

func (m Model) tabBar() string {
	names := []string{"Summary"}
	for _, t := range m.data.Tabs {
		names = append(names, t.Title)
	}
	for i, n := range names {
		if i == m.tab {
			names[i] = "[" + n + "]"
		}
	}
	line := frame.Clip(strings.Join(names, " "), m.width)
	if m.th.NoColor {
		return line
	}
	return m.th.Accent.Render(line)
}
