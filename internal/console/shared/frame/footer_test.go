// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package frame

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/tiagovilasboas/downshift/internal/console/shared/theme"
)

var keys = []Key{
	{"↑/↓", "move"}, {"space", "toggle"}, {"a", "all"}, {"n", "none"},
	{"enter", "confirm"}, {"esc", "back"}, {"q", "quit"},
}

func TestFooterFitsAtEveryWidth(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	full := Footer(keys, 0)
	if lipgloss.Width(full) <= 60 {
		t.Fatalf("fixture too short to exercise the cut: %q", full)
	}
	for _, w := range []int{80, 60, 30} {
		got := Footer(keys, w)
		if lipgloss.Width(got) > w {
			t.Errorf("width %d: Footer is %d cells: %q", w, lipgloss.Width(got), got)
		}
		if strings.Contains(got, "\x1b") {
			t.Errorf("width %d: Footer carries an escape under NO_COLOR: %q", w, got)
		}
		cut := lipgloss.Width(full) > w
		if cut != strings.HasSuffix(got, ellipsis) {
			t.Errorf("width %d: cut = %v but got %q", w, cut, got)
		}
	}
}

func TestFooterIsCellSafe(t *testing.T) {
	got := Footer([]Key{{"日本語", "ヘルプテキスト"}}, 8)
	if lipgloss.Width(got) > 8 {
		t.Errorf("wide runes overflow: %q is %d cells", got, lipgloss.Width(got))
	}
	if Footer(nil, 30) != "" {
		t.Error("no keys must give an empty footer")
	}
	if got := Footer(keys, 1); got != ellipsis {
		t.Errorf("width 1 = %q", got)
	}
}

func TestHeader(t *testing.T) {
	got := Header("Setup", "2/4", 20)
	if lipgloss.Width(got) != 20 || !strings.HasPrefix(got, "Setup") || !strings.HasSuffix(got, "2/4") {
		t.Errorf("Header = %q", got)
	}
	if got := Header("Setup", "", 20); got != "Setup" {
		t.Errorf("no step = %q", got)
	}
	if got := Header("A long title here", "2/4", 12); lipgloss.Width(got) > 12 {
		t.Errorf("narrow Header overflows: %q", got)
	}
}

func TestHintsNeverOverflow(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	th := theme.New()
	for _, w := range []int{80, 60, 30} {
		got := Hints(th, Footer(keys, 0), "v0.7.0", w)
		if lipgloss.Width(got) > w {
			t.Errorf("width %d: Hints is %d cells: %q", w, lipgloss.Width(got), got)
		}
	}
	if got := Hints(th, "q quit", "v1", 40); lipgloss.Width(got) < 38 || !strings.HasSuffix(got, "v1") {
		t.Errorf("short hints keep right alignment: %q", got)
	}
}
