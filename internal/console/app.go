// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package console

import (
	"context"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/tiagovilasboas/downshift-ai/internal/console/home"
	"github.com/tiagovilasboas/downshift-ai/internal/console/output"
	"github.com/tiagovilasboas/downshift-ai/internal/console/palette"
	"github.com/tiagovilasboas/downshift-ai/internal/console/shared/command"
	"github.com/tiagovilasboas/downshift-ai/internal/console/shared/frame"
	"github.com/tiagovilasboas/downshift-ai/internal/console/shared/theme"
)

type mode int

const (
	modeScreen mode = iota
	modePalette
	modeConfirm
	modeOutput
)

type ranMsg struct{ res command.Result }

type terminalDoneMsg struct{}

type app struct {
	opts    Options
	run     command.Runner
	theme   theme.Theme
	width   int
	height  int
	mode    mode
	home    home.Model
	palette palette.Model
	out     output.Model
	pending palette.ChosenMsg // the write command waiting for a yes
	last    command.Result    // the most recent captured run
	ctx     context.Context   // the running capture; cancelling it kills the child
	cancel  context.CancelFunc
	ticking bool // a spinner tick is already scheduled
}

func newApp(o Options) app {
	run := command.Runner{Exe: o.Exe}
	a := app{opts: o, run: run, theme: theme.New(), width: 80, height: 24,
		home: home.New(run), palette: palette.New(o.Commands)}
	if len(o.Start) > 0 {
		a.mode, a.out = modeOutput, output.Running(o.Start)
		a.ctx, a.cancel = context.WithCancel(context.Background())
	}
	a.ticking = true // Init starts the tick: home or the start command is loading
	return a
}

func (a app) Init() tea.Cmd {
	if len(a.opts.Start) > 0 {
		return tea.Batch(a.captureCmd(a.ctx, a.opts.Start), frame.Tick())
	}
	return tea.Batch(a.home.Load(), frame.Tick())
}

func (a app) rows() int { return max(a.height-5, 3) }

// Update runs the message and then makes sure a spinner tick is scheduled
// whenever something is loading, and only then.
func (a app) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	m, cmd := a.update(msg)
	next := m.(app)
	if !next.ticking && next.spinning() {
		next.ticking = true
		cmd = tea.Batch(cmd, frame.Tick())
	}
	return next, cmd
}

// spinning reports whether something is loading. A console started on one
// command never loads the home screen.
func (a app) spinning() bool {
	return (len(a.opts.Start) == 0 && a.home.Loading()) || a.out.IsRunning()
}

// cancelRun stops the running command; its result then arrives as a ranMsg.
func (a app) cancelRun() { a.cancel() }

func (a app) update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case frame.TickMsg:
		a.home, a.out = a.home.Advance(), a.out.Advance()
		a.ticking = a.spinning()
		if a.ticking {
			return a, frame.Tick()
		}
		return a, nil
	case tea.WindowSizeMsg:
		a.width, a.height = msg.Width, msg.Height
		return a, nil
	case tea.KeyPressMsg:
		running := a.mode == modeOutput && a.out.IsRunning() && a.cancel != nil
		switch {
		case running && (msg.String() == "ctrl+c" || msg.String() == "esc"):
			a.cancelRun()
			return a, nil
		case msg.String() == "ctrl+c":
			return a, tea.Quit
		}
	case palette.ChosenMsg:
		if msg.Spec.Writes {
			a.mode, a.pending = modeConfirm, msg
			return a, nil
		}
		return a.start(msg)
	case palette.ClosedMsg, output.ClosedMsg:
		if len(a.opts.Start) > 0 {
			return a, tea.Quit // started on one command: there is no home to go back to
		}
		a.mode = modeScreen
		return a, nil
	case output.RerunMsg:
		return a.capture(msg.Args)
	case ranMsg:
		if a.cancel != nil {
			a.cancel()
		}
		a.out, a.last = output.Done(msg.res), msg.res
		return a, nil
	case terminalDoneMsg:
		a.mode = modeScreen
		return a, a.home.Load()
	}
	return a.route(msg)
}

// route sends a message to whatever has focus.
func (a app) route(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch a.mode {
	case modePalette:
		a.palette, cmd = a.palette.Update(msg)
	case modeOutput:
		a.out, cmd = a.out.Update(msg, a.rows())
	case modeConfirm:
		if k, ok := msg.(tea.KeyPressMsg); ok {
			if k.String() == "y" {
				return a.start(a.pending)
			}
			a.mode = modeScreen
		}
	default:
		if k, ok := msg.(tea.KeyPressMsg); ok {
			switch k.String() {
			case "q":
				return a, tea.Quit
			case ":", "ctrl+k":
				a.mode, a.palette = modePalette, palette.New(a.opts.Commands)
				return a, nil
			}
		}
		a.home, cmd = a.home.Update(msg)
	}
	return a, cmd
}

func (a app) start(c palette.ChosenMsg) (tea.Model, tea.Cmd) {
	if c.Spec.Terminal {
		a.mode = modeScreen
		return a, tea.ExecProcess(a.run.Interactive(c.Args), func(error) tea.Msg { return terminalDoneMsg{} })
	}
	return a.capture(c.Args)
}

func (a app) capture(args []string) (tea.Model, tea.Cmd) {
	a.mode, a.out = modeOutput, output.Running(args)
	a.ctx, a.cancel = context.WithCancel(context.Background())
	return a, a.captureCmd(a.ctx, args)
}

func (a app) captureCmd(ctx context.Context, args []string) tea.Cmd {
	run := a.run
	return func() tea.Msg { return ranMsg{res: run.Capture(ctx, args)} }
}

func (a app) View() tea.View {
	t := a.theme
	tab := home.Name
	body, hints := a.home.View(t, a.width), home.Hints
	switch a.mode {
	case modePalette:
		tab = "Commands"
		body, hints = a.palette.View(t, a.rows()-4, a.width), palette.Hints
	case modeOutput:
		tab = "Output"
		body, hints = a.out.View(t, a.rows(), a.width), output.Hints
		if a.out.IsRunning() {
			hints = output.RunningHints
		}
	case modeConfirm:
		tab = "Confirm"
		body, hints = a.confirmView(), "y yes  any other key no"
	}
	header := frame.Tabs(t, []string{tab}, 0, a.width) + "\n" + frame.Rule(t, a.width)
	footer := frame.Rule(t, a.width) + "\n" + frame.Hints(t, hints, a.opts.Version, a.width)
	v := tea.NewView(frame.Screen(header, body, footer, a.height))
	v.AltScreen = true
	return v
}

func (a app) confirmView() string {
	t := a.theme
	name := "downshift " + strings.Join(a.pending.Args, " ")
	return "\n " + t.Warn.Render(theme.SymWarn) + " " + t.Bold.Render(name) +
		" changes files or state.\n\n Run it?  " + t.Selected.Render(" No ") + "   Yes\n"
}
