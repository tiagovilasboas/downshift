// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package confirm

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/tiagovilasboas/downshift-ai/internal/console/shared/theme"
)

var update = flag.Bool("update", false, "rewrite golden files")

func key(m Model, k tea.KeyPressMsg) Model {
	m, _ = m.Update(k)
	return m
}

func newModel(t *testing.T, destructive bool) Model {
	t.Setenv("NO_COLOR", "1")
	body := []string{"write ~/.claude/settings.json", "remove the old downshift hook entry from the shared configuration"}
	return New(theme.New(), "Apply these changes?", body, destructive)
}

func TestDefaultFocus(t *testing.T) {
	m := key(newModel(t, false), tea.KeyPressMsg{Code: tea.KeyEnter})
	if !m.Done() || !m.Yes() {
		t.Errorf("non-destructive enter: done=%v yes=%v", m.Done(), m.Yes())
	}
	m = key(newModel(t, true), tea.KeyPressMsg{Code: tea.KeyEnter})
	if !m.Done() || m.Yes() {
		t.Errorf("destructive enter must be no: done=%v yes=%v", m.Done(), m.Yes())
	}
}

func TestKeys(t *testing.T) {
	cases := []struct {
		name string
		keys []tea.KeyPressMsg
		yes  bool
	}{
		{"y", []tea.KeyPressMsg{{Code: 'y', Text: "y"}}, true},
		{"n", []tea.KeyPressMsg{{Code: 'n', Text: "n"}}, false},
		{"esc", []tea.KeyPressMsg{{Code: tea.KeyEsc}}, false},
		{"right then enter", []tea.KeyPressMsg{{Code: tea.KeyRight}, {Code: tea.KeyEnter}}, false},
		{"tab twice then enter", []tea.KeyPressMsg{{Code: tea.KeyTab}, {Code: tea.KeyTab}, {Code: tea.KeyEnter}}, true},
		{"left then enter", []tea.KeyPressMsg{{Code: tea.KeyLeft}, {Code: tea.KeyEnter}}, false},
	}
	for _, c := range cases {
		m := newModel(t, false)
		for _, k := range c.keys {
			m = key(m, k)
		}
		if !m.Done() || m.Yes() != c.yes {
			t.Errorf("%s: done=%v yes=%v, want yes=%v", c.name, m.Done(), m.Yes(), c.yes)
		}
	}
}

func TestDestructiveTabReachesYes(t *testing.T) {
	m := key(newModel(t, true), tea.KeyPressMsg{Code: tea.KeyTab})
	m = key(m, tea.KeyPressMsg{Code: tea.KeyEnter})
	if !m.Yes() {
		t.Error("tab must move focus to yes")
	}
}

func TestUnknownKeyDoesNotAnswer(t *testing.T) {
	m := key(newModel(t, false), tea.KeyPressMsg{Code: 'x', Text: "x"})
	if m.Done() {
		t.Error("unknown key answered")
	}
}

func TestGolden(t *testing.T) {
	for _, w := range []int{80, 60} {
		for _, d := range []bool{false, true} {
			m, _ := newModel(t, d).Update(tea.WindowSizeMsg{Width: w, Height: 24})
			compare(t, fmt.Sprintf("w%d_destructive_%v.golden", w, d), m.View())
		}
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
