// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package console

import (
	"context"
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/tiagovilasboas/downshift/internal/console/output"
	"github.com/tiagovilasboas/downshift/internal/console/palette"
	"github.com/tiagovilasboas/downshift/internal/console/shared/command"
	"github.com/tiagovilasboas/downshift/internal/console/shared/frame"
)

func sized(a app, w, h int) app {
	m, _ := a.Update(tea.WindowSizeMsg{Width: w, Height: h})
	return m.(app)
}

func TestViewFitsTerminalNoColor(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	long := strings.Repeat("a very long line of output ", 12)
	specs := []command.Spec{{Args: []string{"report"}, Summary: strings.Repeat("summary ", 12)}}
	for _, size := range [][2]int{{80, 24}, {60, 24}} {
		w, h := size[0], size[1]
		a := sized(newApp(Options{Commands: specs, Version: "v1.2.3"}), w, h)
		o, _ := a.Update(ranMsg{res: command.Result{Args: []string{"x"}, Stdout: strings.Repeat(long+"\n", 60)}})
		oa := o.(app)
		oa.mode = modeOutput
		views := map[string]app{"home": a, "palette": press(a, ":"), "output": oa}
		for name, v := range views {
			lines := strings.Split(v.View().Content, "\n")
			if len(lines) > h {
				t.Errorf("%s %dx%d: %d lines, over %d", name, w, h, len(lines), h)
			}
			for _, l := range lines {
				if lw := lipgloss.Width(l); lw > w {
					t.Errorf("%s %dx%d: line is %d cells: %q", name, w, h, lw, l)
				}
			}
		}
	}
}

func TestHeaderTabNamesTheActiveView(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	a := sized(newApp(Options{Commands: []command.Spec{{Args: []string{"report"}}}}), 100, 24)
	if !strings.Contains(a.View().Content, "[1 Home]") {
		t.Error("home tab missing")
	}
	if !strings.Contains(press(a, ":").View().Content, "[1 Commands]") {
		t.Error("palette tab should say Commands")
	}
	m, _ := a.Update(palette.ChosenMsg{Spec: command.Spec{Args: []string{"report"}}, Args: []string{"report"}})
	if !strings.Contains(m.(app).View().Content, "[1 Output]") {
		t.Error("output tab should say Output")
	}
}

func TestEscAndCtrlCCancelRunningCommand(t *testing.T) {
	for _, k := range []tea.KeyPressMsg{{Code: tea.KeyEscape}, {Code: 'c', Mod: tea.ModCtrl}} {
		a := startApp("x")
		if a.ctx.Err() != nil {
			t.Fatal("context cancelled before the key")
		}
		m, cmd := a.Update(k)
		if cmd != nil {
			t.Errorf("%v: cancel must not quit or schedule anything", k)
		}
		if !errors.Is(m.(app).ctx.Err(), context.Canceled) {
			t.Errorf("%v did not cancel the run", k)
		}
		if m.(app).mode != modeOutput {
			t.Errorf("%v left the output pane before the result arrived", k)
		}
	}
}

func TestSpinnerTicksWhileRunningAndStopsWhenDone(t *testing.T) {
	a := startApp("x")
	m, cmd := a.Update(frame.TickMsg{})
	if cmd == nil || !m.(app).ticking {
		t.Fatal("tick must reschedule while running")
	}
	m, _ = m.(app).Update(ranMsg{res: command.Result{Args: []string{"x"}}})
	m, cmd = m.(app).Update(frame.TickMsg{})
	if cmd != nil || m.(app).ticking {
		t.Error("tick must stop once nothing is loading")
	}
	m, cmd = m.(app).Update(output.RerunMsg{Args: []string{"x"}})
	if cmd == nil || !m.(app).ticking {
		t.Error("rerun must restart the spinner tick")
	}
}
