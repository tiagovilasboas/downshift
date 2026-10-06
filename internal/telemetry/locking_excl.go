// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

//go:build !(linux || darwin || freebsd || netbsd || openbsd || dragonfly)

package telemetry

import (
	"fmt"
	"os"
	"time"
)

// tryLock creates "<path>.lock" exclusively and writes the owner PID. An
// existing lock older than staleLockAge is removed as orphaned.
func tryLock(path string) (*FileLock, bool, error) {
	lockPath := path + ".lock"
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err == nil {
		fmt.Fprintf(f, "%d\n", os.Getpid())
		return &FileLock{path: path, lockfd: f, removeOnUnlock: true}, false, nil
	}
	if !os.IsExist(err) {
		return nil, false, err
	}
	if info, statErr := os.Stat(lockPath); statErr == nil && time.Since(info.ModTime()) > staleLockAge {
		_ = os.Remove(lockPath) // orphaned by a killed process; retry next poll
	}
	return nil, true, nil
}

func unlockFD(*os.File) error { return nil }
