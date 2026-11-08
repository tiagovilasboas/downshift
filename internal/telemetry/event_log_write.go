// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package telemetry

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/tiagovilasboas/downshift-ai/internal/paths"
)

// defaultEventPath returns the routing telemetry JSONL path under the state dir.
func defaultEventPath() string {
	if path := os.Getenv("DOWNSHIFT_EVENT_LOG"); path != "" {
		return path
	}
	p, err := paths.EventsPath()
	if err != nil {
		return "events.jsonl"
	}
	return p
}

var mu sync.Mutex

// lockTimeout bounds how long a hook waits for the event-log lock. It stays
// well below every harness hook budget; a lost event is reported, never
// allowed to stall a spawn.
const lockTimeout = 500 * time.Millisecond

// recordErrOut receives the one-line warning when an event cannot be
// written. Tests replace it.
var recordErrOut io.Writer = os.Stderr

// Record appends ev to the default event log. A telemetry failure never
// interrupts a hook, but it is not silent either: one short line goes to
// stderr so a lost event is visible.
func Record(ev Event) {
	canonicalizeModels(&ev)
	if err := AppendTo(defaultEventPath(), ev); err != nil {
		fmt.Fprintf(recordErrOut, "downshift: telemetry event not recorded: %v\n", err)
	}
}

// AppendTo appends ev to the given path, creating the file and parent
// directories as needed. Returns any write error.
//
// Concurrency note: the in-process mutex (mu) prevents data races between
// goroutines in the same binary. Multi-process safety comes from LockFile:
// flock(2) on Unix (released by the kernel when a holder dies, so a killed
// hook cannot orphan it) and a PID lock file with stale detection elsewhere.
// On POSIX systems, once a process owns the lock, O_APPEND writes are
// atomic at the kernel level for sizes < PIPE_BUF (~4 KB), so JSON lines
// will not interleave even under contention.
func AppendTo(path string, ev Event) error {
	mu.Lock()
	defer mu.Unlock()

	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}

	// Acquire exclusive lock (multi-process safe).
	lock, err := LockFile(path, lockTimeout)
	if err != nil {
		return fmt.Errorf("failed to acquire lock on %s: %w", path, err)
	}
	defer lock.Close()

	if _, err := RotationPolicyFromEnv().MaybeRotate(path); err != nil {
		var cleanup *BackupCleanupError
		if !errors.As(err, &cleanup) {
			return fmt.Errorf("log rotation failed for %s: %w", path, err)
		}
		// Rotation worked; stale backups remain. Say so, keep recording.
		fmt.Fprintf(os.Stderr, "downshift: %v\n", err)
	}

	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	// Tighten permissions on existing logs too; OpenFile's mode only applies
	// when creating a file, and event metadata can reveal model usage patterns.
	if err := f.Chmod(0o600); err != nil {
		return err
	}

	line, err := json.Marshal(ev)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(f, "%s\n", line); err != nil {
		return err
	}
	// No fsync: the write(2) is in the OS buffer cache and survives a process
	// crash. On macOS Go's File.Sync is F_FULLFSYNC (~2.5 ms measured), paid on
	// every hook spawn for an advisory log. A power loss can drop the last
	// event; the log is statistics, not a ledger.
	return nil
}
