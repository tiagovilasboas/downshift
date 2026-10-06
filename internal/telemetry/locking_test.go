// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package telemetry_test

import (
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

func TestLockFile_AcquireAndRelease(t *testing.T) {
	tmpdir := t.TempDir()
	path := filepath.Join(tmpdir, "test.txt")

	lock, err := telemetry.LockFile(path, 100*time.Millisecond)
	if err != nil {
		t.Fatalf("LockFile failed: %v", err)
	}
	if lock == nil {
		t.Fatal("expected non-nil lock")
	}

	// Lock file should exist.
	lockPath := path + ".lock"
	if _, err := os.Stat(lockPath); err != nil {
		t.Fatalf("lock file not created: %v", err)
	}

	// Release lock.
	if err := lock.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// After Close the lock must be immediately re-acquirable. (On Unix the
	// lock file stays in place; only the flock on it matters.)
	again, err := telemetry.LockFile(path, 50*time.Millisecond)
	if err != nil {
		t.Fatalf("re-acquire after Close failed: %v", err)
	}
	again.Close()
}

func TestLockFile_ExclusiveAccess(t *testing.T) {
	tmpdir := t.TempDir()
	path := filepath.Join(tmpdir, "test.txt")

	// First goroutine acquires lock.
	lock1, err := telemetry.LockFile(path, 100*time.Millisecond)
	if err != nil {
		t.Fatalf("first LockFile failed: %v", err)
	}
	defer lock1.Close()

	// Second goroutine tries to acquire lock and should timeout.
	done := make(chan error, 1)
	go func() {
		_, err := telemetry.LockFile(path, 50*time.Millisecond)
		done <- err
	}()

	err = <-done
	if err == nil {
		t.Fatal("expected lock acquisition to timeout or fail")
	}
}

func TestLockFile_SerializesWrites(t *testing.T) {
	tmpdir := t.TempDir()
	path := filepath.Join(tmpdir, "test.txt")

	// Launch 5 goroutines that each acquire the lock and write to the file.
	var wg sync.WaitGroup
	var writeCount atomic.Int32
	var writeErr atomic.Pointer[error]

	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			lock, err := telemetry.LockFile(path, 1*time.Second)
			if err != nil {
				writeErr.Store(&err)
				return
			}
			defer lock.Close()

			// Write to file while holding lock.
			f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
			if err != nil {
				writeErr.Store(&err)
				return
			}
			defer f.Close()

			if _, err := f.WriteString("line\n"); err != nil {
				writeErr.Store(&err)
				return
			}
			writeCount.Add(1)
		}(i)
	}

	wg.Wait()

	if writeErr.Load() != nil {
		t.Fatalf("write failed: %v", *writeErr.Load())
	}

	if writeCount.Load() != 5 {
		t.Fatalf("expected 5 writes, got %d", writeCount.Load())
	}

	// Verify file content: 5 lines, no corruption.
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}
	lines := 0
	for _, c := range content {
		if c == '\n' {
			lines++
		}
	}
	if lines != 5 {
		t.Fatalf("expected 5 newlines, got %d", lines)
	}
}

func TestLockFile_CreatesDirIfNeeded(t *testing.T) {
	tmpdir := t.TempDir()
	path := filepath.Join(tmpdir, "subdir", "nested", "test.txt")

	lock, err := telemetry.LockFile(path, 100*time.Millisecond)
	if err != nil {
		t.Fatalf("LockFile failed: %v", err)
	}
	defer lock.Close()

	// Parent directory should be created.
	if _, err := os.Stat(filepath.Dir(path)); err != nil {
		t.Fatalf("parent directory not created: %v", err)
	}
}
