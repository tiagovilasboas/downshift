// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package session

import (
	"time"

	"github.com/tiagovilasboas/downshift/internal/quotamodel"
)

// LoadQuotaHook reads the canonical quota cache for one harness.
// core wires it to the hook quota registers, so this package does not import
// core. A nil hook is missing evidence.
var LoadQuotaHook func(harness string) (*quotamodel.Snapshot, bool)

// LoadNativeQuotaHook reads an opt-in native export before the cache.
// core wires it to the hook quota registers. A nil hook is missing evidence.
var LoadNativeQuotaHook func(harness string) (*quotamodel.Snapshot, bool)

// LoadNativeSessionHook reads a harness-native session export. core registers
// it because the path comes from harness capabilities. A nil hook skips the
// native layer. Presence is authoritative: the bool is true even when the
// returned list is unknown.
var LoadNativeSessionHook func(harness string, now time.Time) (SessionList, bool)
