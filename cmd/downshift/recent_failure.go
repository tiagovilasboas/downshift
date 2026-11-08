// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"time"

	"github.com/tiagovilasboas/downshift-ai/internal/telemetry"
)

// recentFailureTailBytes bounds the event-log read the hook makes to find a
// recent failed downshift: the respawn window is minutes, so the tail holds it.
const recentFailureTailBytes int64 = 512 << 10

// recentFailedDownshift reports whether a spawn Downshift moved down in this
// session ended in failure within the respawn window. No session, an
// unreadable log or no such failure is false: the class keeps routing.
func recentFailedDownshift(session string, now time.Time) bool {
	if session == "" {
		return false
	}
	events, err := telemetry.ReadEventsTail(recentFailureTailBytes)
	if err != nil {
		return false
	}
	return telemetry.FailedDownshiftWithin(events, session, now)
}
