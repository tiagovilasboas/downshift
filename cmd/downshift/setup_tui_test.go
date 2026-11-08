// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tiagovilasboas/downshift-ai/internal/console/setupwizard"
	"github.com/tiagovilasboas/downshift-ai/internal/setup"
	"github.com/tiagovilasboas/downshift-ai/internal/testenv"
)

func TestSetupTUI_NoHarnessesDetected(t *testing.T) {
	var stdout, stderr bytes.Buffer
	opts := setup.Options{
		Home: t.TempDir(), In: strings.NewReader(""), Out: &stdout, Err: &stderr, Now: time.Now, TUI: true,
	}
	if rc := setupTUI(opts); rc != 1 {
		t.Fatalf("setupTUI() = %d, want 1 when no harnesses are found", rc)
	}
	if !strings.Contains(stderr.String(), "no supported harness found") {
		t.Errorf("stderr = %q; want 'no supported harness found'", stderr.String())
	}
}

func tuiOpts(t *testing.T, in io.Reader, out, errw *bytes.Buffer) setup.Options {
	t.Helper()
	home := t.TempDir()
	// Keep shared state (context optimizer config) out of the real home.
	testenv.SetHome(t, home)
	if err := os.MkdirAll(filepath.Join(home, ".claude"), 0o755); err != nil {
		t.Fatal(err)
	}
	return setup.Options{
		Home: home, Binary: "/usr/local/bin/downshift", StateDir: filepath.Join(home, ".downshift"),
		In: in, Out: out, Err: errw, Now: time.Now, TUI: true,
		NoPlugin: true, SkipDiscovery: true,
	}
}

func settingsWritten(o setup.Options) bool {
	_, err := os.Stat(filepath.Join(o.Home, ".claude", "settings.json"))
	return err == nil
}

// fakeWizard stands in for the Bubble Tea program: it runs the injected
// preview like the review step does, then returns the scripted outcome.
func fakeWizard(t *testing.T, sel setupwizard.Selection, confirmed bool, err error) func(setupwizard.Options, io.Reader, io.Writer) (setupwizard.Selection, bool, error) {
	t.Helper()
	return func(o setupwizard.Options, _ io.Reader, _ io.Writer) (setupwizard.Selection, bool, error) {
		if len(o.Harnesses) == 0 || len(o.Components) == 0 {
			t.Error("wizard opened without harnesses or components")
		}
		if o.Preview != nil {
			files, perr := o.Preview(sel)
			if perr != nil || len(files) == 0 {
				t.Errorf("preview = %v, %v", files, perr)
			}
		}
		return sel, confirmed, err
	}
}

func withWizard(t *testing.T, run func(setupwizard.Options, io.Reader, io.Writer) (setupwizard.Selection, bool, error)) {
	t.Helper()
	oldRun, oldTTY := setupWizardRun, setupHasTTY
	setupWizardRun, setupHasTTY = run, func(io.Reader) bool { return true }
	t.Cleanup(func() { setupWizardRun, setupHasTTY = oldRun, oldTTY })
}

var claudeSelection = setupwizard.Selection{Harnesses: []string{"claude-code"}, Components: []string{"hooks", "skills", "steering"}}

func TestSetupTUI_ConfirmedAppliesOnceAndPrintsTheCommand(t *testing.T) {
	var out, errw bytes.Buffer
	o := tuiOpts(t, strings.NewReader(""), &out, &errw)
	withWizard(t, fakeWizard(t, claudeSelection, true, nil))
	if settingsWritten(o) {
		t.Fatal("precondition: nothing written yet")
	}
	if rc := setupTUI(o); rc != 0 {
		t.Fatalf("rc = %d (stderr: %s)", rc, errw.String())
	}
	if !settingsWritten(o) {
		t.Errorf("settings not written:\n%s", out.String())
	}
	s := out.String()
	if strings.Contains(s, "would change") {
		t.Errorf("the preview must not reach the output:\n%s", s)
	}
	if n := strings.Count(s, "+++ b/"+"settings.json"); n > 1 {
		t.Errorf("settings diff printed %d times, want once:\n%s", n, s)
	}
	if !strings.Contains(s, "downshift setup claude-code --no-mcp --no-plugin --no-mods --no-session-policy --no-policy --keep-binaries") {
		t.Errorf("equivalent command missing:\n%s", s)
	}
}

func TestSetupTUI_CancelAndErrorWriteNothing(t *testing.T) {
	cases := []struct {
		name string
		err  error
		code int
		msg  string
	}{
		{"cancelled", nil, 0, "Nothing was written"},
		{"wizard error", errors.New("boom"), 1, "boom"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var out, errw bytes.Buffer
			o := tuiOpts(t, strings.NewReader(""), &out, &errw)
			withWizard(t, fakeWizard(t, claudeSelection, false, c.err))
			if rc := setupTUI(o); rc != c.code {
				t.Fatalf("rc = %d, want %d", rc, c.code)
			}
			if settingsWritten(o) || !strings.Contains(out.String()+errw.String(), c.msg) {
				t.Errorf("want no write and %q:\n%s%s", c.msg, out.String(), errw.String())
			}
		})
	}
}

func TestSetupTUI_WithoutTerminalNeverWritesUnasked(t *testing.T) {
	for _, tc := range []struct {
		name      string
		in        io.Reader
		wantWrite bool
	}{
		{"nil input", nil, false},
		{"empty input", strings.NewReader(""), false},
		{"unknown answer", strings.NewReader("maybe\n"), false},
		{"explicit y", strings.NewReader("y\n"), true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var out, errw bytes.Buffer
			o := tuiOpts(t, tc.in, &out, &errw)
			setupWizardRun = func(setupwizard.Options, io.Reader, io.Writer) (setupwizard.Selection, bool, error) {
				t.Error("the wizard must not open without a terminal")
				return setupwizard.Selection{}, false, nil
			}
			t.Cleanup(func() { setupWizardRun = setupwizard.Run })
			setupTUI(o)
			if got := settingsWritten(o); got != tc.wantWrite {
				t.Errorf("settings written = %v, want %v\n%s", got, tc.wantWrite, out.String())
			}
		})
	}
}

func TestSetupRunYesNeverPrompts(t *testing.T) {
	var out, errw bytes.Buffer
	o := tuiOpts(t, strings.NewReader(""), &out, &errw)
	o.Yes, o.TUI = true, false
	if rc := setupRun(o); rc != 0 {
		t.Fatalf("rc = %d (stderr: %s)", rc, errw.String())
	}
	if strings.Contains(out.String(), "[y/N]") || !settingsWritten(o) {
		t.Errorf("--yes must write without prompting:\n%s", out.String())
	}
}

func TestIsInteractiveTerminal(t *testing.T) {
	if isInteractiveTerminal(nil) {
		t.Error("nil reader reported as interactive terminal")
	}
	if isInteractiveTerminal(strings.NewReader("not a terminal")) {
		t.Error("strings.Reader reported as interactive terminal")
	}
	if isInteractiveTerminal(&bytes.Buffer{}) {
		t.Error("bytes.Buffer reported as interactive terminal")
	}
}

func TestWantsSetupWizard(t *testing.T) {
	cases := []struct {
		name string
		o    setup.Options
		tty  bool
		want bool
	}{
		{"bare setup in a terminal", setup.Options{}, true, true},
		{"named harness in a terminal", setup.Options{Names: []string{"cursor"}}, true, true},
		{"no terminal", setup.Options{}, false, false},
		{"--tui without a terminal", setup.Options{TUI: true}, false, true},
		{"--yes", setup.Options{Yes: true}, true, false},
		{"--dry-run", setup.Options{DryRun: true}, true, false},
		{"--undo", setup.Options{Undo: true}, true, false},
		{"--no-tui", setup.Options{NoTUI: true}, true, false},
		{"--yes with --tui", setup.Options{Yes: true, TUI: true}, false, false},
		{"--dry-run with --tui", setup.Options{DryRun: true, TUI: true}, false, false},
	}
	for _, c := range cases {
		if got := wantsSetupWizard(c.o, c.tty); got != c.want {
			t.Errorf("%s: wantsSetupWizard = %v, want %v", c.name, got, c.want)
		}
	}
}
