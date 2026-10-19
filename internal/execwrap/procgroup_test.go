// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

//go:build !windows

package execwrap

import (
	"bytes"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// A stop reaches the whole process group: the grandchild a test binary would
// leave behind dies with the signal.
func TestRun_SignalReachesGrandchildren(t *testing.T) {
	pidFile := filepath.Join(t.TempDir(), "pid")
	fakeBin(t, "go", "sleep 30 & echo $! > "+pidFile+"; wait")
	sig := make(chan os.Signal, 1)
	go func() {
		time.Sleep(500 * time.Millisecond)
		sig <- syscall.SIGTERM
	}()
	var out, errb bytes.Buffer
	r := Run([]string{"go", "test"}, Options{Stdin: strings.NewReader(""), Stdout: &out, Stderr: &errb, Compact: true, Signals: sig})
	if r.ExitClass != ExitClassSignal {
		t.Fatalf("result = %+v", r)
	}
	raw, err := os.ReadFile(pidFile)
	if err != nil {
		t.Fatal(err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(raw)))
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if syscall.Kill(pid, 0) != nil {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	_ = syscall.Kill(pid, syscall.SIGKILL)
	t.Fatal("grandchild survived the stop")
}
