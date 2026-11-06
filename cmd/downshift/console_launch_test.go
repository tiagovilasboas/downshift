// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/console"
	"github.com/tiagovilasboas/downshift/internal/console/surface"
)

func TestLaunchDecision(t *testing.T) {
	env := func(m map[string]string) func(string) string { return func(k string) string { return m[k] } }
	facts := func(args []string, e map[string]string, in, out bool) surface.Facts {
		return surface.Facts{Args: args, StdinTTY: in, StdoutTTY: out, Getenv: env(e)}
	}
	cases := []struct {
		name string
		f    surface.Facts
		want bool
	}{
		{"human session", facts([]string{"session"}, nil, true, true), true},
		{"piped stdout", facts([]string{"session"}, nil, true, false), false},
		{"piped stdin", facts([]string{"session"}, nil, false, true), false},
		{"json flag", facts([]string{"session", "--json"}, nil, true, true), false},
		{"plain flag", facts([]string{"session", "--plain"}, nil, true, true), false},
		{"plain env", facts([]string{"session"}, map[string]string{"DOWNSHIFT_PLAIN": "1"}, true, true), false},
		{"ci", facts([]string{"session"}, map[string]string{"CI": "true"}, true, true), false},
		{"hook", facts([]string{"claude-code"}, nil, true, true), false},
		{"unknown", facts([]string{"nope"}, nil, true, true), false},
	}
	for _, c := range cases {
		if got := surface.Decide(c.f, consoleCommands).Console; got != c.want {
			t.Errorf("%s: console=%v, want %v", c.name, got, c.want)
		}
	}
}

func TestStripPlain(t *testing.T) {
	in := []string{"session", "--plain"}
	if got := stripPlain(in); !slices.Equal(got, []string{"session"}) {
		t.Errorf("got %v", got)
	}
	if !slices.Equal(in, []string{"session", "--plain"}) {
		t.Error("input mutated")
	}
}

func TestFlushOutcome(t *testing.T) {
	var out, errw bytes.Buffer
	code := flushOutcome(console.Outcome{Stdout: "o", Stderr: "e", Code: 3}, &out, &errw)
	if code != 3 || out.String() != "o" || errw.String() != "e" {
		t.Errorf("code=%d out=%q err=%q", code, out.String(), errw.String())
	}
}

// TestLaunchPlainFlagMatchesGolden: `session --plain` piped equals the golden
// for `session`, so handlers never see the flag.
func TestLaunchPlainFlagMatchesGolden(t *testing.T) {
	// Own binary: plainBinary's is removed with whichever test built it first.
	bin := filepath.Join(t.TempDir(), "downshift")
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}
	if out, err := exec.Command("go", "build", "-o", bin, ".").CombinedOutput(); err != nil {
		t.Fatalf("go build: %v\n%s", err, out)
	}
	saved := binPath // normalisePlain reads the shared path
	binPath = bin
	t.Cleanup(func() { binPath = saved })
	home, emptyPath := t.TempDir(), t.TempDir()
	cmd := exec.Command(bin, "session", "--plain")
	cmd.Env = plainEnv(home, emptyPath)
	cmd.Dir = home
	var out, errOut bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errOut
	code := 0
	if err := cmd.Run(); err != nil {
		ee, ok := err.(*exec.ExitError)
		if !ok {
			t.Fatal(err)
		}
		code = ee.ExitCode()
	}
	got := fmt.Sprintf("exit: %d\n--- stdout ---\n%s--- stderr ---\n%s",
		code, normalisePlain(out.String(), home, emptyPath), normalisePlain(errOut.String(), home, emptyPath))
	want, err := os.ReadFile(filepath.Join("testdata", "plain", "session.golden"))
	if err != nil {
		t.Fatal(err)
	}
	if string(want) != got {
		t.Errorf("session --plain differs from golden\n--- want ---\n%s\n--- got ---\n%s", want, got)
	}
}
