// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package home

import (
	"errors"
	"flag"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

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

const doctorJSON = `{"checks":[{"id":"hooks","status":"fail","detail":"hook for Claude Code is missing from settings.json and cannot route","fix":"run downshift setup --yes to install the hooks"},{"id":"café","status":"warn","detail":"ünïcödé ünïcödé ünïcödé ünïcödé ünïcödé ünïcödé ünïcödé ünïcödé ünïcödé ünïcödé ünïcödé"},{"id":"a","status":"ok"}]}`

func loaded(t *testing.T) Model {
	t.Helper()
	m, _ := New(command.Runner{}).Update(loadedMsg{
		session: command.Result{Stdout: "model sonnet  spawns 3\nsaved 1200 tokens in this session so far today\n"},
		doctor:  command.Result{Code: 1, Stdout: doctorJSON},
	})
	return m
}

func TestViewGoldenNoColor(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	m := loaded(t)
	for name, width := range map[string]int{"view_80x24": 80, "view_60x24": 60} {
		out := m.View(theme.New(), width)
		for _, l := range strings.Split(out, "\n") {
			if w := lipgloss.Width(l); w > width {
				t.Errorf("%s: line is %d cells, over %d: %q", name, w, width, l)
			}
		}
		golden(t, name, out)
	}
}

func TestLoadingViewGolden(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	m := New(command.Runner{})
	first := m.View(theme.New(), 80)
	golden(t, "loading_80x24", first)
	if m.Advance().View(theme.New(), 80) == first {
		t.Error("spinner did not change after Advance")
	}
}

func TestCommandErrorIsShownNotEmptyState(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	m, _ := New(command.Runner{}).Update(loadedMsg{
		session: command.Result{Err: errors.New("fork/exec /nope: no such file or directory")},
		doctor:  command.Result{Err: errors.New("boom")},
	})
	out := m.View(theme.New(), 80)
	if strings.Contains(out, "No subagent spawns") {
		t.Errorf("error shown as empty state:\n%s", out)
	}
	if !strings.Contains(out, "session did not load: fork/exec") {
		t.Errorf("session error missing:\n%s", out)
	}
}

func TestOneLineKeepsUTF8Valid(t *testing.T) {
	s := strings.Repeat("é世界", 60)
	for limit := 5; limit < 40; limit++ {
		got := oneLine(s, limit)
		if !strings.HasSuffix(got, "…") || lipgloss.Width(got) > limit {
			t.Errorf("limit %d: %q is %d cells", limit, got, lipgloss.Width(got))
		}
		if strings.ContainsRune(got, '\uFFFD') {
			t.Errorf("limit %d split a rune: %q", limit, got)
		}
	}
}
