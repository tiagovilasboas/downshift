// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package core

import (
	"time"

	"github.com/tiagovilasboas/downshift/internal/quotamodel"
)

// LoadQuotaHook reads the canonical quota cache for one harness.
// quota registers it at init so core does not import the filesystem package.
// A nil hook is missing evidence.
var LoadQuotaHook func(harness string) (*quotamodel.Snapshot, bool)

// LoadNativeQuotaHook reads an opt-in native export before the cache.
// quota registers it at init. A nil hook is missing evidence.
var LoadNativeQuotaHook func(harness string) (*quotamodel.Snapshot, bool)

// ParseNativeQuotaHook decodes a native usage payload into a snapshot.
// quota registers it at init. A nil hook fails closed.
var ParseNativeQuotaHook func(source string, data []byte, observed time.Time, ttl time.Duration) (quotamodel.Snapshot, error)

// RegisterNativeQuotaHook records a harness's native export env and source.
// quota registers it at init. capabilities publishes the table through it.
var RegisterNativeQuotaHook func(harness, env, source string)

// PublishNativeQuotaSources hands each harness's native export to the
// registered quota hook. quota calls this from init after assigning
// RegisterNativeQuotaHook, because this package's init runs first.
func PublishNativeQuotaSources() {
	if RegisterNativeQuotaHook == nil {
		return
	}
	for _, n := range AllHarnessCaps() {
		RegisterNativeQuotaHook(n.Harness, n.Caps.NativeSessionEnv, n.Caps.NativeUsageSource)
	}
}
