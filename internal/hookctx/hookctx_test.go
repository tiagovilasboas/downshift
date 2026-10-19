// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package hookctx_test

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/tiagovilasboas/downshift/internal/hookctx"
)

func needSh(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("uses POSIX helper commands")
	}
	if _, err := exec.LookPath("sleep"); err != nil {
		t.Skip("sleep not available")
	}
}

// A hung helper is killed at the global hook deadline even when its own
// timeout is longer.
func TestRunCommand_GlobalDeadlineKillsHungHelper(t *testing.T) {
	needSh(t)
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	defer hookctx.Set(ctx)()
	start := time.Now()
	_, err := hookctx.RunCommand("sleep 10", "", 5*time.Second)
	if err == nil {
		t.Fatal("hung helper must fail")
	}
	if d := time.Since(start); d > time.Second {
		t.Fatalf("helper ran %s past a 200ms deadline", d)
	}
}

// After the deadline, helpers are not started at all.
func TestRunCommand_ExpiredDeadlineSkipsHelper(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	defer hookctx.Set(ctx)()
	if _, err := hookctx.RunCommand("definitely-not-run", "", time.Second); !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
}

// Output beyond the cap is rejected instead of buffered without bound.
func TestRunCommand_OutputIsCapped(t *testing.T) {
	needSh(t)
	if _, err := exec.LookPath("yes"); err != nil {
		t.Skip("yes not available")
	}
	start := time.Now()
	_, err := hookctx.RunCommand("yes", "", 3*time.Second)
	if err == nil {
		t.Fatal("unbounded output must fail")
	}
	if d := time.Since(start); d > 2*time.Second {
		t.Fatalf("capped helper took %s", d)
	}
}

// A helper whose background child keeps stdout open does not hold the hook:
// WaitDelay (100 ms) bounds the wait.
//
// The first run of a freshly written script can take seconds on a loaded
// machine (the OS vets a new executable; observed 2.07-2.13 s on macOS under
// load while the test passed 30/30 alone). That is process start-up, not the
// runner, so the assertion compares against the same script without a
// background child: the child may cost at most 1 s more than the baseline, and
// the whole run must stay well under the 5 s timeout a real hang would hit.
func TestRunCommand_BackgroundChildDoesNotHang(t *testing.T) {
	needSh(t)
	// Other packages set hookctx during parallel go test ./...; pin Background
	// so this test only measures RunCommand behaviour.
	t.Cleanup(hookctx.Set(context.Background()))
	dir := t.TempDir()
	write := func(name, body string) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(body), 0o700); err != nil {
			t.Fatal(err)
		}
		return p
	}
	// Redirect the background child's stdout/stderr. If they inherit the hook
	// pipe, cmd.Run can block until the command timeout (~5s) even after the
	// parent shell exits.
	withChild := write("child.sh", "#!/bin/sh\nsleep 10 >/dev/null 2>&1 &\necho '{}'\n")
	// The harder case: the child keeps the hook's stdout open. Only WaitDelay
	// ends the wait, so this is the one that fails if the delay is lost.
	holdsPipe := write("holds.sh", "#!/bin/sh\nsleep 10 &\necho '{}'\n")
	baseline := write("plain.sh", "#!/bin/sh\necho '{}'\n")

	timed := func(script string) time.Duration {
		start := time.Now()
		if _, err := hookctx.RunCommand(script, "", 5*time.Second); err != nil {
			t.Fatalf("RunCommand(%s): %v", filepath.Base(script), err)
		}
		return time.Since(start)
	}
	base := timed(baseline)
	_ = timed(withChild) // first run of this script: absorbs one-off start-up cost
	got := timed(withChild)
	if got > base+time.Second || got > 4*time.Second {
		t.Fatalf("background child held the helper for %s (baseline %s, timeout 5s)", got, base)
	}

	start := time.Now()
	_, err := hookctx.RunCommand(holdsPipe, "", 5*time.Second)
	held := time.Since(start)
	if err != nil && !errors.Is(err, exec.ErrWaitDelay) {
		t.Fatalf("RunCommand(holds.sh): %v", err)
	}
	if held > base+time.Second || held > 4*time.Second {
		t.Fatalf("a child holding stdout kept the hook for %s (baseline %s, WaitDelay should end it in ~100ms)", held, base)
	}
}

func TestRunCommand_ReturnsOutput(t *testing.T) {
	needSh(t)
	out, err := hookctx.RunCommand("cat", "hello", time.Second)
	if err != nil || string(out) != "hello" {
		t.Fatalf("out=%q err=%v", out, err)
	}
}
