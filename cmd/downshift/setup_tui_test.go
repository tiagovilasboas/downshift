// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSetupTUI_NoHarnessesDetected(t *testing.T) {
	tempHome := t.TempDir()
	var stdout, stderr bytes.Buffer

	opts := setupOptions{
		Home: tempHome,
		In:   strings.NewReader(""),
		Out:  &stdout,
		Err:  &stderr,
		Now:  time.Now,
		TUI:  true,
	}

	rc := setupTUI(opts)
	if rc != 1 {
		t.Fatalf("setupTUI() = %d, want 1 when no harnesses are found", rc)
	}
	errOut := stderr.String()
	if !strings.Contains(errOut, "no supported harness found") {
		t.Errorf("setupTUI() stderr = %q; want 'no supported harness found'", errOut)
	}
}

func tuiOpts(t *testing.T, in io.Reader, out, errw *bytes.Buffer) setupOptions {
	t.Helper()
	home := t.TempDir()
	// Keep shared state (context optimizer config) out of the real home.
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	if err := os.MkdirAll(filepath.Join(home, ".claude"), 0o755); err != nil {
		t.Fatal(err)
	}
	return setupOptions{
		Home: home, Binary: "/usr/local/bin/downshift", StateDir: filepath.Join(home, ".downshift"),
		In: in, Out: out, Err: errw, Now: time.Now, TUI: true,
		NoPlugin: true, SkipDiscovery: true,
	}
}

func settingsWritten(o setupOptions) bool {
	_, err := os.Stat(filepath.Join(o.Home, ".claude", "settings.json"))
	return err == nil
}

func TestSetupTUI_FirstStepAbort(t *testing.T) {
	var out, errw bytes.Buffer
	o := tuiOpts(t, strings.NewReader("n\n"), &out, &errw)
	if rc := setupTUI(o); rc != 0 {
		t.Fatalf("rc = %d, want 0", rc)
	}
	if !strings.Contains(out.String(), "Setup aborted by user") || settingsWritten(o) {
		t.Errorf("want abort and no write:\n%s", out.String())
	}
}

func TestSetupTUI_ConfirmDefaultIsNo(t *testing.T) {
	cases := []struct {
		name      string
		input     string
		wantWrite bool
	}{
		{"empty answer aborts", "\n\n", false},
		{"EOF at confirm aborts", "\n", false},
		{"EOF everywhere aborts", "", false},
		{"unknown answer aborts", "\nmaybe\n", false},
		{"explicit n aborts", "\nn\n", false},
		{"y applies", "\ny\n", true},
		{"s applies", "\ns\n", true},
		{"sim applies", "\nSim\n", true},
		{"yes applies", "\nyes\n", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var out, errw bytes.Buffer
			o := tuiOpts(t, strings.NewReader(tc.input), &out, &errw)
			if rc := setupTUI(o); rc != 0 {
				t.Fatalf("rc = %d, want 0 (stderr: %s)", rc, errw.String())
			}
			if got := settingsWritten(o); got != tc.wantWrite {
				t.Errorf("settings written = %v, want %v\n%s", got, tc.wantWrite, out.String())
			}
			if !tc.wantWrite && !strings.Contains(out.String(), "Nothing was written") {
				t.Errorf("abort must say nothing was written:\n%s", out.String())
			}
		})
	}
}

func TestSetupTUI_DiffShownBeforeQuestion(t *testing.T) {
	var out, errw bytes.Buffer
	o := tuiOpts(t, strings.NewReader("\n\n"), &out, &errw)
	if rc := setupTUI(o); rc != 0 {
		t.Fatalf("rc = %d", rc)
	}
	s := out.String()
	diff := strings.Index(s, "would change")
	ask := strings.Index(s, "[y/N]")
	if diff < 0 || ask < 0 || diff > ask {
		t.Errorf("diff (%d) must print before the confirm question (%d):\n%s", diff, ask, s)
	}
	if !strings.Contains(s, "+++") {
		t.Errorf("exact diff missing:\n%s", s)
	}
}

func TestSetupTUI_DryRunNeverAsksToApply(t *testing.T) {
	var out, errw bytes.Buffer
	o := tuiOpts(t, strings.NewReader("\ns\n"), &out, &errw)
	o.DryRun = true
	if rc := setupTUI(o); rc != 0 {
		t.Fatalf("rc = %d", rc)
	}
	if strings.Contains(out.String(), "[y/N]") || settingsWritten(o) {
		t.Errorf("dry run must not ask or write:\n%s", out.String())
	}
}

func TestSetupTUI_NonInteractiveNeverPrompts(t *testing.T) {
	for _, tc := range []struct {
		name string
		mut  func(*setupOptions)
	}{
		{"nil input (pipe closed)", func(o *setupOptions) { o.In = nil }},
		{"--yes through setupRun", func(o *setupOptions) { o.Yes = true; o.TUI = false }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var out, errw bytes.Buffer
			o := tuiOpts(t, strings.NewReader(""), &out, &errw)
			tc.mut(&o)
			run := setupTUI
			if o.Yes {
				run = setupRun
			}
			if rc := run(o); rc != 0 {
				t.Fatalf("rc = %d (stderr: %s)", rc, errw.String())
			}
			if strings.Contains(out.String(), "[y/N]") || strings.Contains(out.String(), "[Y/n]") {
				t.Errorf("must not prompt:\n%s", out.String())
			}
			if !settingsWritten(o) {
				t.Errorf("expected settings written:\n%s", out.String())
			}
		})
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
