// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package telemetry

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// A leftover lock file with no live holder (a hook killed mid-append) must
// not block later writers.
func TestLockFile_LeftoverLockFileDoesNotBlock(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.jsonl")
	if err := os.WriteFile(path+".lock", nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS == "windows" {
		old := time.Now().Add(-time.Minute)
		_ = os.Chtimes(path+".lock", old, old)
	}
	start := time.Now()
	lock, err := LockFile(path, lockTimeout)
	if err != nil {
		t.Fatalf("leftover lock file blocked the writer: %v", err)
	}
	lock.Close()
	if d := time.Since(start); d > 300*time.Millisecond {
		t.Fatalf("acquiring past a leftover lock took %s", d)
	}
}

// TestHelperHoldLock is re-executed as a child process by
// TestLockFile_KilledHolderReleasesLock. It takes the lock and waits to be
// killed.
func TestHelperHoldLock(t *testing.T) {
	path := os.Getenv("DOWNSHIFT_TEST_HOLD_LOCK")
	if path == "" {
		t.Skip("helper process only")
	}
	if _, err := LockFile(path, time.Second); err != nil {
		os.Exit(3)
	}
	os.WriteFile(path+".held", []byte("1"), 0o600)
	time.Sleep(time.Minute)
}

// A holder killed with SIGKILL (harness hook timeout) must not leave the
// lock held.
func TestLockFile_KilledHolderReleasesLock(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("flock semantics are Unix-only")
	}
	path := filepath.Join(t.TempDir(), "events.jsonl")
	cmd := exec.Command(os.Args[0], "-test.run=^TestHelperHoldLock$")
	cmd.Env = append(os.Environ(), "DOWNSHIFT_TEST_HOLD_LOCK="+path)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		if _, err := os.Stat(path + ".held"); err == nil {
			break
		}
		if time.Now().After(deadline) {
			cmd.Process.Kill()
			t.Fatal("helper never took the lock")
		}
		time.Sleep(10 * time.Millisecond)
	}
	if _, err := LockFile(path, 50*time.Millisecond); err == nil {
		cmd.Process.Kill()
		t.Fatal("lock must be exclusive while the helper holds it")
	}
	cmd.Process.Kill()
	cmd.Wait()
	lock, err := LockFile(path, lockTimeout)
	if err != nil {
		t.Fatalf("lock still held after its owner was killed: %v", err)
	}
	lock.Close()
}

// A held lock makes Record give up within the short timeout and say so on
// stderr instead of dropping the event silently.
func TestRecord_LockTimeoutIsReported(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.jsonl")
	t.Setenv("DOWNSHIFT_EVENT_LOG", path)
	holder, err := LockFile(path, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer holder.Close()

	var buf bytes.Buffer
	old := recordErrOut
	recordErrOut = &buf
	defer func() { recordErrOut = old }()

	start := time.Now()
	Record(Event{Harness: "claude-code", Outcome: OutcomeAllow})
	if d := time.Since(start); d > 2*lockTimeout {
		t.Fatalf("Record waited %s, want about %s", d, lockTimeout)
	}
	if !strings.Contains(buf.String(), "telemetry event not recorded") {
		t.Fatalf("expected a stderr warning, got %q", buf.String())
	}
}
