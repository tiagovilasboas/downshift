// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package learning

import (
	"time"

	"github.com/tiagovilasboas/downshift-ai/internal/telemetry"
)

// lockWait is how long Record waits for another process before it gives up
// and skips the observation. Learning is advisory; the command never waits
// longer than this for it.
var lockWait = 200 * time.Millisecond

// lockStore takes the cross-process lock beside the store (flock on Unix, an
// O_EXCL file with stale-lock removal elsewhere, both in telemetry.FileLock).
// ok is false when the lock could not be taken in time or at all.
func lockStore() (unlock func(), ok bool) {
	p, err := StorePath()
	if err != nil {
		return nil, false
	}
	l, err := telemetry.LockFile(p, lockWait)
	if err != nil {
		return nil, false
	}
	return func() { _ = l.Unlock() }, true
}
