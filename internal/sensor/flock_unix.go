// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

//go:build unix

package sensor

import (
	"errors"
	"os"
	"syscall"
	"time"
)

// lockTimeout bounds the wait for the sensor lock. The sensor runs on every
// PostToolUse; a held lock (a stuck or stopped holder) must cost one lost
// observation, never stall the hook. Same budget as the event log.
const lockTimeout = 500 * time.Millisecond

func lockFile(f *os.File) error {
	deadline := time.Now().Add(lockTimeout)
	for {
		err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
		if err == nil {
			return nil
		}
		if !errors.Is(err, syscall.EWOULDBLOCK) {
			return err
		}
		if time.Now().After(deadline) {
			return errors.New("sensor lock busy")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func unlockFile(f *os.File) error {
	return syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
}
