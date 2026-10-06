// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package telemetry

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// lockPollInterval is how often a waiting process retries the lock.
const lockPollInterval = 5 * time.Millisecond

// FileLock is an exclusive, multi-process lock guarding one log file.
//
// On Unix it is an flock(2) on "<path>.lock": the kernel releases it when the
// holder exits for any reason (including SIGKILL from a harness hook
// timeout), so a crashed hook can never leave a lock that blocks later hooks.
// The lock file itself is left in place; only the flock matters.
//
// On other platforms it falls back to an O_EXCL lock file holding the owner
// PID. A lock file older than staleLockAge is treated as orphaned (the
// critical section is a single append) and removed before retrying.
type FileLock struct {
	path   string
	lockfd *os.File
	// removeOnUnlock is true for the O_EXCL fallback, where the file's
	// existence is the lock.
	removeOnUnlock bool
}

// staleLockAge bounds how long an O_EXCL fallback lock may be held before it
// is considered orphaned. Appends take milliseconds.
const staleLockAge = 2 * time.Second

// LockFile acquires an exclusive lock for path, waiting at most timeout.
func LockFile(path string, timeout time.Duration) (*FileLock, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	deadline := time.Now().Add(timeout)
	for {
		lock, busy, err := tryLock(path)
		if err != nil {
			return nil, err
		}
		if !busy {
			return lock, nil
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("could not acquire lock on %s (timeout %s)", path, timeout)
		}
		time.Sleep(lockPollInterval)
	}
}

// Unlock releases the lock.
func (l *FileLock) Unlock() error {
	if l == nil || l.lockfd == nil {
		return nil
	}
	err := unlockFD(l.lockfd)
	l.lockfd.Close()
	l.lockfd = nil
	if l.removeOnUnlock {
		if rmErr := os.Remove(l.path + ".lock"); rmErr != nil && !os.IsNotExist(rmErr) {
			return rmErr
		}
	}
	return err
}

// Close releases the lock (alias of Unlock).
func (l *FileLock) Close() error {
	return l.Unlock()
}
