// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

// Package session is the allowlist a hook may write for Downshift by
// Tiago Vilas Boas, https://github.com/tiagovilasboas/downshift-ai.
//
// It loads the list and reports token budget. It does not choose a model.
package session

import (
	"strings"
	"time"

	"github.com/tiagovilasboas/downshift-ai/internal/quotamodel"
)

// SessionModel describes one exact selectable ID. An absent tier means
// unknown; omission must never imply small capability. A per-model list price
// the payload may still carry (input_per_million / output_per_million) is
// ignored: the product uses no price.
type SessionModel struct {
	ID      string   `json:"id"`
	Tier    string   `json:"tier,omitempty"`
	Efforts []string `json:"efforts,omitempty"`
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
	// FromCatalog marks a list built from the catalog's session candidates
	// for a harness whose payload carries none, cut at the current model.
	FromCatalog bool
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

// RestrictToAvailable keeps only the ids that OpenCode reports as selectable
// right now (a "provider/id" pair). A session id matches an available entry
// exactly, or when the entry minus its provider segment (up to the first
// slash) is the id, so a bare id and an id that itself contains slashes
// ("deepseek/deepseek-v4.1-flash" under "openrouter/...") both restrict
// correctly. A bare id matches the entry of any provider. An empty available
// list empties the session, which means "no route" rather than "route to
// anything".
func (s SessionList) RestrictToAvailable(available []string) SessionList {
	if !s.Known {
		return s
	}
	set := make(map[string]bool, len(available))
	for _, id := range available {
		set[id] = true
	}
	var kept []string
	for _, id := range s.IDs {
		if set[id] {
			kept = append(kept, id)
			continue
		}
		// The id matches an entry whose provider segment is dropped.
		for _, avail := range available {
			if i := strings.IndexByte(avail, '/'); i >= 0 && avail[i+1:] == id {
				kept = append(kept, id)
				break
			}
		}
	}
	s.IDs = kept
	return s
}
