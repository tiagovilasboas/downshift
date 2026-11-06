// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package execwrap

import (
	"bufio"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

// rotateLockWait bounds how long an append waits for the rotation lock.
const rotateLockWait = 50 * time.Millisecond

// LogRotateBytes is the size past which an exec log (exec-events.jsonl,
// exec-skips.jsonl) moves to path+".1" before the next append. One older
// generation is kept; the one before it is dropped.
const LogRotateBytes = 10 << 20

const rotatedSuffix = ".1"

// rotateMu makes the size check and the rename one step inside a process, so
// concurrent appenders cannot each rotate the fresh file over the old one.
// Across processes a race can still drop one generation, never cut a line.
var rotateMu sync.Mutex

// appendLine appends one short line (an atomic O_APPEND write), rotating the
// log first when it passed the cap. A writer that raced a rotation and still
// holds the renamed file appends to the older generation; a line is never cut
// in two. Two rotations racing may drop one generation, never corrupt a line.
func appendLine(path string, line []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	rotateIfLarge(path, LogRotateBytes)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(append(line, '\n'))
	return err
}

// rotateIfLarge renames path to path+".1" when it is larger than limit.
// Best effort: a failure leaves the log growing, which is still correct.
func rotateIfLarge(path string, limit int64) {
	rotateMu.Lock()
	defer rotateMu.Unlock()
	if !overLimit(path, limit) {
		return
	}
	lock, err := telemetry.LockFile(path, rotateLockWait)
	if err != nil {
		return // another process is rotating, or the lock is unavailable: append anyway
	}
	defer func() { _ = lock.Unlock() }()
	// Re-check under the lock: another process may have rotated already, and
	// renaming its fresh file over .1 would drop a generation.
	if overLimit(path, limit) {
		_ = os.Rename(path, path+rotatedSuffix)
	}
}

func overLimit(path string, limit int64) bool {
	info, err := os.Stat(path)
	return err == nil && info.Size() > limit
}

// scanLog calls fn for every line of the log: the rotated generation first,
// then the current file, so totals do not drop at a rotation. A rotation
// between the two reads can under-count for a moment, never double count. A missing file
// is empty, not an error.
func scanLog(path string, fn func(line []byte)) error {
	for _, p := range []string{path + rotatedSuffix, path} {
		if err := scanFile(p, fn); err != nil {
			return err
		}
	}
	return nil
}

func scanFile(path string, fn func(line []byte)) error {
	f, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		fn(sc.Bytes())
	}
	return sc.Err()
}
