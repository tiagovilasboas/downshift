// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package session

import (
	"time"

	"github.com/tiagovilasboas/downshift/internal/quotamodel"
)

// WithUsageQuota attaches current provider evidence. A supplied snapshot,
// cached snapshot (even malformed), or required mode closes the quota gate.
// Missing evidence in legacy mode retains routing without claiming credit.
func (s SessionList) WithUsageQuota(harness string, supplied *quotamodel.Snapshot) SessionList {
	s.QuotaTime = time.Now()
	s.QuotaRequired = quotaModeRequired()
	if supplied != nil {
		s.Usage, s.QuotaRequired = supplied, true
		return s
	}
	// Native availability and quota were read from the same bounded export.
	// Preserve that observation rather than reopening a file that may rotate.
	if s.NativeAvailability {
		s.QuotaRequired = true
		return s
	}
	var present bool
	s.Usage, present = loadUsage(harness)
	// A snapshot that expired more than quotamodel.StaleGrace ago is treated as
	// absent, like a user without quota. Required mode keeps holding.
	if present && s.Usage != nil && !s.QuotaRequired && s.Usage.Abandoned(s.QuotaTime) {
		s.Usage, present = nil, false
	}
	s.QuotaRequired = s.QuotaRequired || present
	return s
}

// loadUsage reads the provider evidence for a harness. An explicitly configured
// native export is fresher than the canonical cache. Invalid native input
// remains present and therefore closes the gate instead of falling back to a
// stale cache.
func loadUsage(harness string) (*quotamodel.Snapshot, bool) {
	if LoadNativeQuotaHook != nil {
		if s, present := LoadNativeQuotaHook(harness); present {
			return s, true
		}
	}
	if LoadQuotaHook == nil {
		return nil, false
	}
	return LoadQuotaHook(harness)
}

// StaleQuotaHold reports whether a loaded snapshot for harness has expired but
// still holds model rewrites, and for how long it has been expired. It follows
// the same rule as WithUsageQuota, so doctor and the statusline never disagree
// with the gate. Malformed evidence is not "stale" and reports false.
func StaleQuotaHold(harness string, now time.Time) (expiredFor time.Duration, held bool) {
	s, present := loadUsage(harness)
	if !present || s == nil || s.Validate() != nil || now.Before(s.ExpiresAt) {
		return 0, false
	}
	if !quotaModeRequired() && s.Abandoned(now) {
		return 0, false
	}
	return now.Sub(s.ExpiresAt), true
}

// QuotaStatus reports token budget. A usage snapshot stays authoritative:
// file or hook marks must not turn it into a false Available. When Usage
// is nil, Included is available, Exhausted is exhausted, and every other
// id is unknown. Allowlist membership is not a balance claim.
func (s SessionList) QuotaStatus(harness, id string) quotamodel.Status {
	if s.Usage == nil {
		if s.IsIncluded(id) {
			return quotamodel.Available
		}
		if s.Blocks(id) {
			return quotamodel.Exhausted
		}
		return quotamodel.Unknown
	}
	now := s.QuotaTime
	if now.IsZero() {
		now = time.Now()
	}
	return s.Usage.Evaluate(harness, id, now)
}

// QuotaAllows reports whether the quota gate permits a write of id.
// Required mode, or any present evidence, allows only an available budget.
func (s SessionList) QuotaAllows(harness, id string) bool {
	return !s.QuotaRequired || s.QuotaStatus(harness, id) == quotamodel.Available
}
