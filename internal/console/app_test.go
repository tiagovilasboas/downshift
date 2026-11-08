// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package console

import (
	"fmt"
	"os"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/tiagovilasboas/downshift-ai/internal/console/output"
	"github.com/tiagovilasboas/downshift-ai/internal/console/palette"
	"github.com/tiagovilasboas/downshift-ai/internal/console/shared/command"
)

func press(a app, s string) app {
	k := tea.KeyPressMsg{Code: []rune(s)[0], Text: s}
	m, _ := a.Update(k)
	return m.(app)
}

func TestWriteCommandDefaultsToNo(t *testing.T) {
	a := newApp(Options{Commands: []command.Spec{{Args: []string{"setup"}, Writes: true}}})
	m, _ := a.Update(palette.ChosenMsg{Spec: a.opts.Commands[0], Args: []string{"setup"}})
	a = m.(app)
	if a.mode != modeConfirm {
		t.Fatalf("mode = %v, want confirm", a.mode)
	}
	a = press(a, "n")
	if a.mode != modeScreen {
		t.Fatalf("after n mode = %v, want screen", a.mode)
	}
}

func startApp(args ...string) app {
	return newApp(Options{Exe: os.Args[0], Start: args})
}

func TestStartOpensOutputAndCaptures(t *testing.T) {
	t.Setenv("CONSOLE_HELPER", "1")
	a := startApp("-test.run=TestHelperProcess")
	if a.mode != modeOutput {
		t.Fatalf("mode = %v, want output", a.mode)
	}
	if a.Init() == nil {
		t.Fatal("Init issued no capture")
	}
	m, _ := a.Update(a.captureCmd(a.ctx, a.opts.Start)())
	a = m.(app)
	if a.last.Stdout != "out\n" || a.last.Stderr != "err\n" || a.last.Code != 3 {
		t.Fatalf("last = %+v", a.last)
	}
}

func TestStartQuitsWhenOutputCloses(t *testing.T) {
	for _, key := range []string{"q", "esc"} {
		a := startApp("x")
		m, _ := a.Update(ranMsg{res: command.Result{Args: []string{"x"}}})
		k := tea.KeyPressMsg{Code: []rune(key)[0], Text: key}
		if key == "esc" {
			k = tea.KeyPressMsg{Code: tea.KeyEscape}
		}
		_, cmd := m.Update(k)
		if cmd == nil {
			t.Fatalf("%s: no command", key)
		}
		_, quit := m.Update(cmd())
		if quit == nil {
			t.Fatalf("%s: closing did not quit", key)
		}
		if _, ok := quit().(tea.QuitMsg); !ok {
			t.Fatalf("%s: got %T, want QuitMsg", key, quit())
		}
	}
}

func TestStartRerunIssuesCapture(t *testing.T) {
	a := startApp("x")
	m, _ := a.Update(output.RerunMsg{Args: []string{"x"}})
	if m.(app).mode != modeOutput {
		t.Fatal("rerun must stay in output")
	}
}

func TestBareConsoleKeepsHome(t *testing.T) {
	m, _ := newApp(Options{}).Update(output.ClosedMsg{})
	if m.(app).mode != modeScreen {
		t.Fatal("bare console must return home")
	}
}

func TestHelperProcess(t *testing.T) {
	if os.Getenv("CONSOLE_HELPER") != "1" {
		return
	}
	fmt.Fprintln(os.Stdout, "out")
	fmt.Fprintln(os.Stderr, "err")
	os.Exit(3)
}

func TestColonOpensPalette(t *testing.T) {
	a := press(newApp(Options{}), ":")
	if a.mode != modePalette {
		t.Fatalf("mode = %v, want palette", a.mode)
	}
	if a.View().Content == "" {
		t.Fatal("empty view")
	}
}
