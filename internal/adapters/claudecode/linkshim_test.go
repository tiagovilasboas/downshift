// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package claudecode

import (
	"time"

	"github.com/tiagovilasboas/downshift/internal/childend"
	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

// linkTargetBy keeps the linking tests of this package on the shared
// implementation (internal/childend), fixed to this harness.
func linkTargetBy(prior []telemetry.Event, sessionHash, callHash, modelID string, now time.Time, linkedBy string) *telemetry.Event {
	return childend.LinkTargetBy(harnessID, prior, sessionHash, callHash, modelID, now, linkedBy)
}
