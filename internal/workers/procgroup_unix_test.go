// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

//go:build !windows

package workers

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// A cancel or a budget expiry must stop what the worker started (test runs,
// builds), not only the worker: a program left running keeps spending the
// machine's time and can keep writing.
func TestKillStopsTheProgramsTheWorkerStarted(t *testing.T) {
	pidfile := filepath.Join(t.TempDir(), "child.pid")
	shim(t, "claude", `sleep 30 & echo $! > "`+pidfile+`"; wait`)
	proc, err := ReadOnlyLaunchers()["claude"].Launch(Spec{Agent: "claude", Dir: t.TempDir(), Task: "x", Budget: time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	var child int
	waitFor(t, "the worker to start its child", func() bool {
		b, err := os.ReadFile(pidfile)
		if err != nil {
			return false
		}
		n, err := strconv.Atoi(strings.TrimSpace(string(b)))
		child = n
		return err == nil && n > 0
	})
	if err := proc.Kill(); err != nil {
		t.Fatalf("kill: %v", err)
	}
	waitFor(t, "the worker's child to stop", func() bool { return syscall.Kill(child, 0) != nil })
	_ = proc.Wait()
}
