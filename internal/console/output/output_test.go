// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package output

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/tiagovilasboas/downshift-ai/internal/console/shared/command"
	"github.com/tiagovilasboas/downshift-ai/internal/console/shared/theme"
)

var update = flag.Bool("update", false, "rewrite golden files")

var sgr = regexp.MustCompile("\x1b\\[[0-9;]*m")

// golden compares got, with style escapes removed, to testdata/<name>.golden.
func golden(t *testing.T, name, got string) {
	t.Helper()
	got = sgr.ReplaceAllString(got, "")
	path := filepath.Join("testdata", name+".golden")
	if *update {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if got != string(want) {
		t.Errorf("%s differs from golden:\n--- got ---\n%s\n--- want ---\n%s", name, got, want)
	}
}

func key(s string) tea.KeyPressMsg {
	if s == "esc" {
		return tea.KeyPressMsg{Code: tea.KeyEscape}
	}
	return tea.KeyPressMsg{Code: []rune(s)[0], Text: s}
}

func numbered(n int) command.Result {
	var b strings.Builder
	for i := 1; i <= n; i++ {
		fmt.Fprintf(&b, "line %d %s\n", i, strings.Repeat("wide ", 30))
	}
	return command.Result{Args: []string{"report", "--days=7"}, Stdout: b.String()}
}

func TestViewGoldenNoColor(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	m := Done(numbered(40))
	for name, width := range map[string]int{"view_80x24": 80, "view_60x24": 60} {
		out := m.View(theme.New(), 19, width)
		for _, l := range strings.Split(out, "\n") {
			if w := lipgloss.Width(l); w > width {
				t.Errorf("%s: line is %d cells, over %d: %q", name, w, width, l)
			}
		}
		golden(t, name, out)
	}
}

func TestScrollReachesLastLine(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	const rows = 10
	m := Done(numbered(25))
	m, _ = m.Update(key("G"), rows)
	out := m.View(theme.New(), rows, 200)
	if !strings.Contains(out, "line 25 ") {
		t.Errorf("last line unreachable after G:\n%s", out)
	}
	if !strings.Contains(out, "17–25 of 25") {
		t.Errorf("scroll position missing:\n%s", out)
	}
	for i := 0; i < 40; i++ {
		m, _ = m.Update(key("j"), rows)
	}
	if !strings.Contains(m.View(theme.New(), rows, 200), "line 25 ") {
		t.Error("last line lost after scrolling down past the end")
	}
	if got := len(strings.Split(strings.TrimRight(m.View(theme.New(), rows, 200), "\n"), "\n")); got != rows {
		t.Errorf("view is %d lines, want %d", got, rows)
	}
}

func TestNoPositionWhenEverythingFits(t *testing.T) {
	out := Done(command.Result{Stdout: "one\ntwo\n"}).View(theme.New(), 10, 80)
	if strings.Contains(out, " of ") {
		t.Errorf("position shown for short output:\n%s", out)
	}
}

func TestSpinnerAdvances(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	m := Running([]string{"report"})
	a := m.View(theme.New(), 10, 80)
	if b := m.Advance().View(theme.New(), 10, 80); a == b {
		t.Error("spinner frame did not change")
	}
	if !m.IsRunning() || Done(command.Result{}).IsRunning() {
		t.Error("IsRunning wrong")
	}
}

func TestCanceledResultReadsAsCanceled(t *testing.T) {
	m := Done(command.Result{Args: []string{"x"}, Err: context.Canceled})
	if out := m.View(theme.New(), 10, 80); !strings.Contains(out, "canceled") {
		t.Errorf("canceled missing:\n%s", out)
	}
	m = Done(command.Result{Args: []string{"x"}, Err: errors.New("boom")})
	if out := m.View(theme.New(), 10, 80); !strings.Contains(out, "boom") {
		t.Errorf("error missing:\n%s", out)
	}
}
