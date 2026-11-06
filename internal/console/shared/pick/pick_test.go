// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package pick

import (
	"flag"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

var update = flag.Bool("update", false, "rewrite the golden files")

func items() []Item {
	return []Item{
		{ID: "claude", Label: "Claude Code", Hint: "hooks and skills", Selected: true},
		{ID: "codex", Label: "Codex", Hint: "hooks"},
		{ID: "kiro", Label: "Kiro", Disabled: true, DisabledReason: "not installed"},
		{ID: "cursor", Label: "Cursor", Hint: "rules only", Selected: true},
		{ID: "gemini", Label: "Gemini CLI"},
		{ID: "windsurf", Label: "Windsurf", Hint: "a hint that is long enough to be clipped at narrow widths"},
	}
}

func key(code rune, text string) tea.KeyPressMsg { return tea.KeyPressMsg{Code: code, Text: text} }

var (
	keyUp    = key(tea.KeyUp, "")
	keyDown  = key(tea.KeyDown, "")
	keyEnter = key(tea.KeyEnter, "")
	keyEsc   = key(tea.KeyEscape, "")
	keySpace = key(' ', " ")
)

func runMulti(m MultiSelect, msgs ...tea.Msg) MultiSelect {
	for _, msg := range msgs {
		m, _ = m.Update(msg)
	}
	return m
}

func runList(l List, msgs ...tea.Msg) List {
	for _, msg := range msgs {
		l, _ = l.Update(msg)
	}
	return l
}

func TestListChoosesWithEnter(t *testing.T) {
	l := runList(NewList(items()), keyDown, keyEnter)
	if l.Cancelled() || !l.Done() || l.Result() != "codex" {
		t.Fatalf("done=%v cancelled=%v result=%q", l.Done(), l.Cancelled(), l.Result())
	}
}

func TestListRefusesDisabledAndMovesWithKJ(t *testing.T) {
	l := runList(NewList(items()), keyDown, key('j', "j"), keyEnter)
	if l.Done() {
		t.Fatal("enter on a disabled item must not confirm")
	}
	l = runList(l, key('k', "k"), keyEnter)
	if !l.Done() || l.Result() != "codex" {
		t.Fatalf("done=%v result=%q, want codex", l.Done(), l.Result())
	}
}

func TestListEscCancels(t *testing.T) {
	l := runList(NewList(items()), keyEsc, keyEnter)
	if !l.Cancelled() || l.Done() || l.Result() != "" {
		t.Fatalf("cancelled=%v done=%v result=%q", l.Cancelled(), l.Done(), l.Result())
	}
}

func TestMultiPreselectionToggleAndOrder(t *testing.T) {
	m := NewMultiSelect(items())
	want := []string{"claude", "cursor"}
	if got := m.Result(); !reflect.DeepEqual(got, want) {
		t.Fatalf("preselection = %v, want %v", got, want)
	}
	m = runMulti(m, keySpace, keyDown, keySpace, keyEnter)
	if got, want := m.Result(), []string{"codex", "cursor"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("result = %v, want %v", got, want)
	}
	if !m.Done() || m.Cancelled() {
		t.Fatalf("done=%v cancelled=%v", m.Done(), m.Cancelled())
	}
}

func TestMultiDisabledCannotToggleOrSelect(t *testing.T) {
	has := func(ids []string, id string) bool {
		for _, x := range ids {
			if x == id {
				return true
			}
		}
		return false
	}
	in := items()
	m := runMulti(NewMultiSelect(in), keyDown, keyDown, keySpace)
	if has(m.Result(), "kiro") {
		t.Fatal("space toggled a disabled item")
	}
	m = runMulti(m, key('a', "a"))
	if has(m.Result(), "kiro") {
		t.Fatal("select all picked a disabled item")
	}
	if got, want := m.Result(), []string{"claude", "codex", "cursor", "gemini", "windsurf"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("after a = %v, want %v", got, want)
	}
	// A locked-on item (always on) stays selected through space and n.
	in = items()
	in[2].Selected = true
	m = runMulti(NewMultiSelect(in), keyDown, keyDown, keySpace, key('n', "n"))
	if got := m.Result(); !reflect.DeepEqual(got, []string{"kiro"}) {
		t.Fatalf("locked-on item: got %v, want [kiro]", got)
	}
}

func TestMultiAllNoneAndEsc(t *testing.T) {
	m := runMulti(NewMultiSelect(items()), key('n', "n"))
	if len(m.Result()) != 0 {
		t.Fatalf("after n = %v", m.Result())
	}
	m = runMulti(m, key('a', "a"), keyEsc, keySpace)
	if !m.Cancelled() || m.Done() {
		t.Fatalf("cancelled=%v done=%v", m.Cancelled(), m.Done())
	}
	if len(m.Result()) != 5 {
		t.Fatalf("keys after esc must be ignored, got %v", m.Result())
	}
}

func TestMultiUpdateDoesNotAliasEarlierCopy(t *testing.T) {
	before := NewMultiSelect(items())
	_ = runMulti(before, key('n', "n"))
	if len(before.Result()) != 2 {
		t.Fatalf("earlier copy changed: %v", before.Result())
	}
}

func TestScrollKeepsFocusVisible(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	m := NewMultiSelect(items())
	m.SetSize(80, 3)
	for i := 0; i < 5; i++ {
		m = runMulti(m, keyDown)
		view := m.View()
		if !strings.Contains(view, ">") {
			t.Fatalf("step %d: focused row not visible:\n%s", i, view)
		}
		if got := strings.Count(view, "[ ]") + strings.Count(view, "[x]"); got != 3 {
			t.Fatalf("step %d: %d item rows, want 3", i, got)
		}
	}
	if !strings.Contains(m.View(), "Windsurf") || strings.Contains(m.View(), "Claude Code") {
		t.Fatalf("viewport did not scroll to the end:\n%s", m.View())
	}
	m = runMulti(m, keyUp, keyUp, keyUp, keyUp, keyUp, keyUp)
	if !strings.Contains(m.View(), "Claude Code") {
		t.Fatalf("viewport did not scroll back:\n%s", m.View())
	}
}

func TestNoColorMarkersAndClipping(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	m := NewMultiSelect(items())
	for _, w := range []int{80, 60, 30} {
		m.SetSize(w, 0)
		view := m.View()
		if strings.Contains(view, "\x1b") {
			t.Errorf("width %d: escape under NO_COLOR", w)
		}
		for _, line := range strings.Split(view, "\n") {
			if lipgloss.Width(line) > w {
				t.Errorf("width %d: line is %d cells: %q", w, lipgloss.Width(line), line)
			}
		}
	}
	view := m.View()
	for _, want := range []string{"> [x] Claude Code", "[ ] Codex", "(not installed)"} {
		if !strings.Contains(view, want) {
			t.Errorf("view lacks %q:\n%s", want, view)
		}
	}
}

func TestGoldenViews(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	for _, w := range []int{80, 60} {
		m := NewMultiSelect(items())
		m.SetSize(w, 0)
		golden(t, "multi_"+strconv.Itoa(w), m.View())
		m = runMulti(m, keyDown, keyDown)
		m.SetSize(w, 4)
		golden(t, "multi_scrolled_"+strconv.Itoa(w), m.View())
		l := NewList(items())
		l.SetSize(w, 0)
		golden(t, "list_"+strconv.Itoa(w), l.View())
	}
}

func golden(t *testing.T, name, got string) {
	t.Helper()
	path := filepath.Join("testdata", name+".golden")
	if *update {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(got+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%v (run with -update)", err)
	}
	if string(want) != got+"\n" {
		t.Errorf("%s differs:\n--- want\n%s--- got\n%s\n", name, want, got)
	}
}
