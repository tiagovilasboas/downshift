// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package discovery

import (
	"errors"
	"os"
	"path/filepath"
	"time"
)

// Writers of the cache are rare (`models discover` and the MCP providers tool)
// and hooks only read it, so a lock file taken with O_EXCL is enough to keep
// two processes from losing each other's read-modify-write.
const (
	lockWait  = 2 * time.Second
	lockStale = 10 * time.Second
	lockPoll  = 10 * time.Millisecond
)

// errLockBusy means another writer held the lock for the whole wait.
var errLockBusy = errors.New("discovery cache is locked by another writer")

// lockCache takes the cross-process write lock for the cache at path and
// returns the function that releases it. A lock older than lockStale belongs
// to a crashed writer and is removed.
func lockCache(path string) (func(), error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	lock := path + ".lock"
	deadline := time.Now().Add(lockWait)
	for {
		f, err := os.OpenFile(lock, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err == nil {
			f.Close()
			return func() { os.Remove(lock) }, nil
		}
		if !errors.Is(err, os.ErrExist) {
			return nil, err
		}
		if st, serr := os.Stat(lock); serr == nil && time.Since(st.ModTime()) > lockStale {
			os.Remove(lock)
			continue
		}
		if time.Now().After(deadline) {
			return nil, errLockBusy
		}
		time.Sleep(lockPoll)
	}
}
