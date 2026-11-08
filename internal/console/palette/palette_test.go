// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package palette

import (
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

func specs() []command.Spec {
	return []command.Spec{
		{Args: []string{"report"}, Summary: "Token savings per harness over the last days, with the routing mix and every tier decision"},
		{Args: []string{"setup"}, Summary: "Install hooks, skills and the MCP server for the detected harnesses", Writes: true},
		{Args: []string{"doctor"}, Summary: "Check the install"},
		{Args: []string{"console", "--tui"}, Summary: "Open the console", Terminal: true},
	}
}

func TestViewGoldenNoColor(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	m := New(specs())
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

func TestSelectedRowClippedInColor(t *testing.T) {
	m := New(specs())
	out := m.View(theme.New(), 19, 80)
	for _, l := range strings.Split(out, "\n") {
		if w := lipgloss.Width(l); w > 80 {
			t.Errorf("line is %d cells, over 80: %q", w, l)
		}
	}
}
