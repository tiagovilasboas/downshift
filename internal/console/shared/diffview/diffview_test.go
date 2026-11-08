// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package diffview

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/tiagovilasboas/downshift-ai/internal/console/shared/theme"
)

var update = flag.Bool("update", false, "rewrite golden files")

func sample() []File {
	long := []string{"@@ -1,40 +1,41 @@"}
	for i := 1; i <= 38; i++ {
		long = append(long, fmt.Sprintf(" context line %02d of the settings file with some extra words to make it wide", i))
	}
	long = append(long, "-old last line", "+new last line")
	return []File{
		{Path: "~/.claude/settings.json", Lines: long},
		{Path: "~/.codex/config.toml", Lines: []string{"@@ -1 +1 @@", "-a", "+b"}},
	}
}

func newModel(t *testing.T, w, h int) Model {
	t.Setenv("NO_COLOR", "1")
	m, _ := New(theme.New(), sample()).Update(tea.WindowSizeMsg{Width: w, Height: h})
	return m
}

func press(m Model, codes ...rune) Model {
	for _, c := range codes {
		k := tea.KeyPressMsg{Code: c}
		if c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c == '[' || c == ']' {
			k.Text = string(c)
		}
		m, _ = m.Update(k)
	}
	return m
}

func TestMaxScrollReachesLastLine(t *testing.T) {
	m := press(newModel(t, 80, 12), 'G')
	if !strings.Contains(m.View(), "+new last line") {
		t.Fatalf("G does not show the last line:\n%s", m.View())
	}
	m = press(m, tea.KeyDown, tea.KeyDown, tea.KeyDown)
	if m.top != m.maxTop() || !strings.Contains(m.View(), "line 32–41 of 41") {
		t.Fatalf("scrolled past the end: top=%d\n%s", m.top, m.View())
	}
	m = press(newModel(t, 80, 12), tea.KeyPgDown, tea.KeyPgDown, tea.KeyPgDown, tea.KeyPgDown, tea.KeyPgDown)
	if m.top != m.maxTop() {
		t.Errorf("pgdn top = %d, want %d", m.top, m.maxTop())
	}
	if m = press(m, 'g'); m.top != 0 {
		t.Errorf("g top = %d", m.top)
	}
	if m = press(m, tea.KeyUp); m.top != 0 {
		t.Errorf("up at top = %d", m.top)
	}
}

func TestFileSwitching(t *testing.T) {
	m := press(newModel(t, 80, 12), tea.KeyDown, tea.KeyDown, tea.KeyTab)
	if m.cur != 1 || m.top != 0 || !strings.Contains(m.View(), "file 2/2") {
		t.Fatalf("tab: cur=%d top=%d", m.cur, m.top)
	}
	if m = press(m, ']'); m.cur != 0 {
		t.Errorf("] wraps to first, cur=%d", m.cur)
	}
	if m = press(m, '['); m.cur != 1 {
		t.Errorf("[ wraps to last, cur=%d", m.cur)
	}
}

func TestDone(t *testing.T) {
	for _, c := range []rune{tea.KeyEnter, 'q'} {
		if m := press(newModel(t, 80, 12), c); !m.Done() {
			t.Errorf("key %q did not finish", c)
		}
	}
	if press(newModel(t, 80, 12), tea.KeyDown).Done() {
		t.Error("down finished")
	}
}

func TestEmptyAndShortFile(t *testing.T) {
	m := New(theme.New(), nil)
	if m.View() == "" {
		t.Error("empty viewer renders nothing")
	}
	press(m, tea.KeyDown, 'G', tea.KeyTab)
	m, _ = New(theme.New(), sample()[1:]).Update(tea.WindowSizeMsg{Width: 80, Height: 12})
	if m = press(m, 'G'); m.top != 0 {
		t.Errorf("short file top = %d", m.top)
	}
}

func TestLinesAreClipped(t *testing.T) {
	for _, w := range []int{80, 60, 20} {
		for _, row := range strings.Split(newModel(t, w, 12).View(), "\n") {
			if lipgloss.Width(row) > w {
				t.Errorf("width %d: row is %d cells: %q", w, lipgloss.Width(row), row)
			}
		}
	}
}

func TestNoColorIsPlain(t *testing.T) {
	if strings.Contains(newModel(t, 80, 12).View(), "\x1b") {
		t.Error("escape sequence under NO_COLOR")
	}
}

func TestGolden(t *testing.T) {
	for _, w := range []int{80, 60} {
		m := newModel(t, w, 14)
		compare(t, fmt.Sprintf("w%d_top.golden", w), m.View())
		compare(t, fmt.Sprintf("w%d_bottom.golden", w), press(m, 'G').View())
	}
}

func compare(t *testing.T, name, got string) {
	t.Helper()
	path := filepath.Join("testdata", name)
	if *update {
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(want) != got {
		t.Errorf("%s differs:\n--- want\n%s\n--- got\n%s", name, want, got)
	}
}
