// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: BUSL-1.1
// Commercial use requires a licence — see LICENSE for terms.

//go:build linux || darwin || freebsd || netbsd || openbsd || dragonfly

package telemetry

import (
	"errors"
	"os"
	"syscall"
)

// tryLock takes a non-blocking flock on "<path>.lock". busy is true when
// another process holds it.
func tryLock(path string) (*FileLock, bool, error) {
	f, err := os.OpenFile(path+".lock", os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, false, err
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		f.Close()
		if errors.Is(err, syscall.EWOULDBLOCK) || errors.Is(err, syscall.EAGAIN) || errors.Is(err, syscall.EINTR) {
			return nil, true, nil
		}
		return nil, false, err
	}
	return &FileLock{path: path, lockfd: f}, false, nil
}

func unlockFD(f *os.File) error {
	return syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
}
