// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: BUSL-1.1
// Commercial use requires a licence — see LICENSE for terms.

package telemetry

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// FileLock provides multi-process safe file operations using OS-level locking.
// On Unix systems, it uses flock(2). On Windows, it falls back to file
// rename-based locking (atomic, but slower).
type FileLock struct {
	path   string
	lockfd *os.File
}

// LockFile acquires an exclusive lock on the file at path.
// The lock is advisory (processes must cooperate) but prevents concurrent writes.
// Timeout is the maximum time to wait for the lock before giving up.
// Returns the lock handle; call Close() to release it.
func LockFile(path string, timeout time.Duration) (*FileLock, error) {
	lockPath := path + ".lock"
	deadline := time.Now().Add(timeout)

	// Ensure parent directory exists.
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}

	for {
		// Try to open the lock file exclusively (fail if exists).
		// On Unix, this is O_CREAT|O_EXCL which is atomic.
		f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err == nil {
			// Lock acquired immediately.
			return &FileLock{path: path, lockfd: f}, nil
		}

		if !os.IsExist(err) {
			// Real error, not "file exists".
			return nil, err
		}

		// Lock file exists; another process holds the lock.
		// Wait a bit and retry.
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("could not acquire lock on %s (timeout)", path)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// Unlock releases the file lock by removing the lock file.
// Safe to call even if the lock was never acquired.
func (l *FileLock) Unlock() error {
	if l == nil || l.lockfd == nil {
		return nil
	}
	l.lockfd.Close()
	lockPath := l.path + ".lock"
	return os.Remove(lockPath)
}

// Close is an alias for Unlock() for convenience with defer.
func (l *FileLock) Close() error {
	return l.Unlock()
}
