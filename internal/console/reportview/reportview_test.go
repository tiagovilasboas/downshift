// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package reportview

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

func sample() Data {
	var routing []string
	for i := 1; i <= 20; i++ {
		routing = append(routing, fmt.Sprintf("routing detail line %02d with enough words to be rather wide for a narrow terminal", i))
	}
	return Data{
		Header: Header{Days: 30, Commands: 1652, Compacted: 176},
		Harnesses: []Row{
			{Harness: "claude-code", TokensSaved: 82000, RoutingObserved: true, RoutedSharePct: 69, Billable: 9100000,
				PremiumObserved: true, NonFrontier: 101, TierKnown: 107, PremiumPct: 94, Reworked: 0, Ended: 107},
			{Harness: "codex", TokensSaved: 12500, RawBytes: 60000, SavedBytes: 50000, CutPct: 83, Commands: 40, Compacted: 30, DirectRuns: 40},
		},
		Tabs: []Tab{
			{Title: "Savings", Lines: []string{"saved a", "saved b"}},
			{Title: "Routing", Lines: routing},
			{Title: "Efficiency", Lines: []string{"eff"}},
			{Title: "Quota", Lines: []string{"quota"}},
		},
	}
}

func newModel(t *testing.T, w, h int) Model {
	t.Setenv("NO_COLOR", "1")
	m, _ := New(theme.New(), sample()).Update(tea.WindowSizeMsg{Width: w, Height: h})
	return m
}

func press(m Model, keys ...string) Model {
	for _, k := range keys {
		var msg tea.KeyPressMsg
		switch k {
		case "right":
			msg = tea.KeyPressMsg{Code: tea.KeyRight}
		case "left":
			msg = tea.KeyPressMsg{Code: tea.KeyLeft}
		case "down":
			msg = tea.KeyPressMsg{Code: tea.KeyDown}
		case "pgdown":
			msg = tea.KeyPressMsg{Code: tea.KeyPgDown}
		case "esc":
			msg = tea.KeyPressMsg{Code: tea.KeyEscape}
		default:
			msg = tea.KeyPressMsg{Code: rune(k[0]), Text: k}
		}
		m, _ = m.Update(msg)
	}
	return m
}

func TestTabSwitching(t *testing.T) {
	m := newModel(t, 80, 12)
	if m.tab != 0 || !strings.Contains(m.View(), "[Summary]") {
		t.Fatalf("starts on summary:\n%s", m.View())
	}
	if m = press(m, "right"); m.tab != 1 {
		t.Errorf("right: tab = %d", m.tab)
	}
	if m = press(m, "left", "left"); m.tab != 4 {
		t.Errorf("left wraps: tab = %d", m.tab)
	}
	if m = press(m, "3"); m.tab != 2 || !strings.Contains(m.View(), "[Routing]") {
		t.Errorf("3 jumps to Routing: tab = %d", m.tab)
	}
	if m = press(m, "5", "9"); m.tab != 4 {
		t.Errorf("5 jumps to Quota, 9 is ignored: tab = %d", m.tab)
	}
	if m = press(m, "1"); m.tab != 0 {
		t.Errorf("1 jumps to Summary: tab = %d", m.tab)
	}
}

func TestScrollReachesLastLine(t *testing.T) {
	m := press(newModel(t, 80, 12), "3", "G")
	v := m.View()
	if !strings.Contains(v, "line 20") || !strings.Contains(v, "12–20 of 20") {
		t.Fatalf("G does not show the last line:\n%s", v)
	}
	if press(m, "down", "down").top != m.maxTop() {
		t.Error("scrolled past the end")
	}
	m = press(newModel(t, 80, 12), "3", "pgdown", "pgdown", "pgdown")
	if m.top != m.maxTop() {
		t.Errorf("pgdn top = %d, want %d", m.top, m.maxTop())
	}
	if m = press(m, "g"); m.top != 0 {
		t.Errorf("g top = %d", m.top)
	}
	if m = press(m, "down", "right"); m.top != 0 {
		t.Errorf("switching tab resets scroll, top = %d", m.top)
	}
}

func TestQuit(t *testing.T) {
	for _, k := range []string{"q", "esc"} {
		if !press(newModel(t, 80, 12), k).Done() {
			t.Errorf("%s did not finish", k)
		}
	}
	if press(newModel(t, 80, 12), "right", "down").Done() {
		t.Error("navigation finished the screen")
	}
}

func TestEmptyData(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	m := New(theme.New(), Data{})
	m = press(m, "right", "down", "G")
	if !strings.Contains(m.View(), "No harness activity") {
		t.Errorf("empty view:\n%s", m.View())
	}
}

func TestUnobservedNeverShowsZeroOrBar(t *testing.T) {
	v := newModel(t, 80, 40).View()
	idx := strings.Index(v, "codex ")
	if idx < 0 {
		t.Fatal("codex missing")
	}
	block := v[idx:]
	// A harness with Compactor runs but no subagent evidence says so in one
	// line: never a routing share, a 0% or a bar for what was not measured.
	if !strings.Contains(block, "no subagent spawns measured") || strings.Contains(block, "On a cheaper model") {
		t.Errorf("unobserved routing drew figures:\n%s", block)
	}
	if !strings.Contains(block, "~12k*") || !strings.Contains(block, "40 · 30 compacted") {
		t.Errorf("codex Compactor figures missing:\n%s", block)
	}
}

func TestNarrowDropsBars(t *testing.T) {
	v := newModel(t, 50, 24).View()
	if strings.ContainsAny(v, "█░") || !strings.Contains(v, "69%") {
		t.Errorf("narrow keeps numbers and drops bars:\n%s", v)
	}
}

func TestEveryLineFitsWidth(t *testing.T) {
	for _, w := range []int{100, 80, 60, 40, 20} {
		for tab := 1; tab <= 5; tab++ {
			m := press(newModel(t, w, 12), fmt.Sprint(tab))
			for _, row := range strings.Split(m.View(), "\n") {
				if lipgloss.Width(row) > w {
					t.Errorf("width %d tab %d: row is %d cells: %q", w, tab, lipgloss.Width(row), row)
				}
			}
		}
	}
}

func TestNoColorIsPlain(t *testing.T) {
	if strings.Contains(newModel(t, 80, 24).View(), "\x1b") {
		t.Error("escape sequence under NO_COLOR")
	}
}

func TestHuman(t *testing.T) {
	cases := map[int64]string{
		0: "0", 950: "950", 999: "999", 1000: "1k", 1500: "1.5k", 9999: "10k", 82000: "82k",
		82400: "82k", 999499: "999k", 999999: "1M", 1000000: "1M", 9100000: "9.1M",
		12000000: "12M", 2500000000: "2.5B", -1500: "-1.5k",
	}
	for in, want := range cases {
		if got := Human(in); got != want {
			t.Errorf("Human(%d) = %q, want %q", in, got, want)
		}
	}
}

func TestGolden(t *testing.T) {
	for _, w := range []int{80, 60} {
		m := newModel(t, w, 16)
		compare(t, fmt.Sprintf("w%d_summary.golden", w), m.View())
		compare(t, fmt.Sprintf("w%d_routing.golden", w), press(m, "3").View())
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
