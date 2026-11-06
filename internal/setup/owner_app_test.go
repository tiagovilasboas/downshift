// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package setup

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// withOwnerAppState pins the running-app seam for one test.
func withOwnerAppState(t *testing.T, s OwnerAppState) {
	t.Helper()
	prev := OwnerAppRunningState
	OwnerAppRunningState = func(string) OwnerAppState { return s }
	t.Cleanup(func() { OwnerAppRunningState = prev })
}

func TestProcessListHas(t *testing.T) {
	list := "/usr/sbin/cfprefsd\n/Applications/Claude.app/Contents/MacOS/Claude\n"
	cli := "/Users/me/Library/Application Support/Claude/claude-code/claude.app/Contents/MacOS/claude\n"
	cases := []struct {
		name, list, app string
		want            bool
	}{
		{"app running", list, "Claude", true},
		{"only the lowercase CLI", cli, "Claude", false},
		{"helper is not the app", "/Applications/Claude.app/Contents/Frameworks/Claude Helper.app/Contents/MacOS/Claude Helper\n", "Claude", false},
		{"bare name line", "  Claude  \n", "Claude", true},
		{"empty list", "", "Claude", false},
	}
	for _, c := range cases {
		if got := processListHas(c.list, c.app); got != c.want {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
}

func TestOwnerAppWarning(t *testing.T) {
	cases := []struct {
		state OwnerAppState
		want  []string
	}{
		{OwnerAppStopped, nil},
		{OwnerAppRunning, []string{"Claude is running", "Cmd+Q", "rerun `downshift setup` from a terminal"}},
		{OwnerAppUnknown, []string{"if it is running", "Cmd+Q", "from a terminal"}},
	}
	for _, c := range cases {
		got := ownerAppWarning("Claude", c.state)
		if c.want == nil && got != "" {
			t.Errorf("state %d: got %q, want empty", c.state, got)
		}
		for _, w := range c.want {
			if !strings.Contains(got, w) {
				t.Errorf("state %d: %q lacks %q", c.state, got, w)
			}
		}
	}
}

func TestSetupClaudeDesktopHoldsWhenAppOwnsFile(t *testing.T) {
	cases := []struct {
		name     string
		state    OwnerAppState
		dry      bool
		wantFile bool
		inOut    string
	}{
		{name: "running holds", state: OwnerAppRunning, inOut: "held"},
		{name: "running dry-run shows hold", state: OwnerAppRunning, dry: true, inOut: "held"},
		{name: "unsupported falls back to write and note", state: OwnerAppUnknown, wantFile: true, inOut: "note: Claude rewrites this file"},
		{name: "not running writes", state: OwnerAppStopped, wantFile: true, inOut: "written"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e := newSetupEnv(t, claudeDesktopDirRel())
			withOwnerAppState(t, c.state)
			args := []string{"claude-desktop", "--yes"}
			if c.dry {
				args = append(args, "--dry-run")
			}
			code, out, errOut := e.desktopRun(args...)
			if code != 0 {
				t.Fatalf("code %d\nout:%s\nerr:%s", code, out, errOut)
			}
			if !strings.Contains(out, c.inOut) {
				t.Errorf("stdout lacks %q:\n%s", c.inOut, out)
			}
			if c.state == OwnerAppRunning && !strings.Contains(out, "Cmd+Q") {
				t.Errorf("hold lacks the fix:\n%s", out)
			}
			_, err := os.Stat(filepath.Join(e.home, claudeDesktopConfigRel))
			if wrote := err == nil && !c.dry; wrote != c.wantFile {
				t.Errorf("file written = %v, want %v", wrote, c.wantFile)
			}
		})
	}
}
