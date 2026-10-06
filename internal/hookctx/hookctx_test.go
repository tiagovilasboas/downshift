// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
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

	"github.com/tiagovilasboas/harness-downshift/internal/hookctx"
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
// WaitDelay bounds the wait.
func TestRunCommand_BackgroundChildDoesNotHang(t *testing.T) {
	needSh(t)
	script := filepath.Join(t.TempDir(), "helper.sh")
	if err := os.WriteFile(script, []byte("#!/bin/sh\nsleep 10 &\necho '{}'\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	hookctx.RunCommand(script, "", 5*time.Second)
	if d := time.Since(start); d > 2*time.Second {
		t.Fatalf("background child held the helper for %s", d)
	}
}

func TestRunCommand_ReturnsOutput(t *testing.T) {
	needSh(t)
	out, err := hookctx.RunCommand("cat", "hello", time.Second)
	if err != nil || string(out) != "hello" {
		t.Fatalf("out=%q err=%v", out, err)
	}
}
