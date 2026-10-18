// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

// Package session is the allowlist a hook may write for Downshift by
// Tiago Vilas Boas, https://github.com/tiagovilasboas/downshift.
//
// It loads the list and reports token budget. It does not choose a model.
package session

import (
	"time"

	"github.com/tiagovilasboas/downshift/internal/quotamodel"
)

// SessionModel describes one exact selectable ID. Absent tier and prices mean
// unknown; omission must never imply small capability or free inference.
type SessionModel struct {
	ID      string   `json:"id"`
	Tier    string   `json:"tier,omitempty"`
	Efforts []string `json:"efforts,omitempty"`
	InputM  *float64 `json:"input_per_million,omitempty"`
	OutputM *float64 `json:"output_per_million,omitempty"`
	Harness string   `json:"harness,omitempty"`
}

// SessionList is the only set of model ids a hook may write.
// The catalog describes tier, cost, family, and effort for ids that are
// also in this list. It must not introduce an id the session does not have.
//
// Known is false when the list could not be determined (hook payload had
// no allowlist and the user file is missing or has no key for the harness).
// Callers must not rewrite the model in that case.
type SessionList struct {
	IDs   []string
	Known bool
	// Metadata is optional native/operator capability evidence for exact IDs.
	Metadata []SessionModel
	// Unordered marks a hook availability inventory without a capability order.
	// Unlike a native quota export, this alone is not quota evidence.
	Unordered bool
	// Inferred marks a list derived from the current model's catalog tier
	// ladder rather than learned from the harness.
	Inferred bool
	// NativeAvailability contains provider IDs without an operator capability
	// order. These candidates require resolver metadata before selection.
	NativeAvailability bool
	// Included ids have a stable token budget in this call. Empty means neither
	// the hook nor this harness's quota object reported one. Allowlist
	// membership is not a credit source.
	Included []string
	// Exhausted ids are in the session but must not be selected. The user
	// has no remaining token budget for them. A hook unavailable_models list
	// or this harness's quota.exhausted list sets this.
	Exhausted []string
	// CreditsReported is true when the hook sent included_models or
	// unavailable_models, or when this harness's quota object supplied
	// included and/or exhausted. The allowlist is not a credit report.
	CreditsReported bool
	// Usage evidence is separate from operator allowlists and hook marks.
	Usage         *quotamodel.Snapshot
	QuotaRequired bool
	QuotaTime     time.Time
}

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

// UnknownSession is the fail-open list: no rewrite.
func UnknownSession() SessionList { return SessionList{} }

// KnownSession marks ids as the session allowlist, including a known-empty list.
func KnownSession(ids []string) SessionList {
	cp := append([]string(nil), ids...)
	return SessionList{IDs: cp, Known: true}
}

// Contains reports whether id is an exact member of a known session list.
func (s SessionList) Contains(id string) bool {
	return s.has(s.IDs, id)
}

// IsIncluded reports an operator mark: this id still has token budget.
func (s SessionList) IsIncluded(id string) bool {
	return s.has(s.Included, id)
}

// Blocks reports an operator mark: this id has no remaining token budget.
func (s SessionList) Blocks(id string) bool {
	return s.has(s.Exhausted, id)
}

func (s SessionList) has(ids []string, id string) bool {
	if !s.Known || id == "" {
		return false
	}
	for _, candidate := range ids {
		if candidate == id {
			return true
		}
	}
	return false
}

// WithHookQuota replaces included and exhausted lists when the hook sent them.
// An absent slice supplies no mark; a present slice is authoritative and
// replaces any mark loaded from quota.<harness>. The allowlist is not
// credit evidence.
func (s SessionList) WithHookQuota(included, exhausted *[]string) SessionList {
	if !s.Known || (included == nil && exhausted == nil) {
		return s
	}
	s.CreditsReported = true
	if included != nil {
		s.Included = append([]string(nil), (*included)...)
	}
	if exhausted != nil {
		s.Exhausted = append([]string(nil), (*exhausted)...)
	}
	return s
}
