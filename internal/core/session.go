// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package core

import (
	"encoding/json"
	"os"
	"time"

	"github.com/tiagovilasboas/downshift/internal/paths"
	"github.com/tiagovilasboas/downshift/internal/quota"
)

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
	// NativeAvailability contains provider IDs without an operator capability
	// order. These candidates require resolver metadata before selection.
	NativeAvailability bool
	// Included ids have a stable token budget in this call. Empty means the
	// hook did not report one. A file on disk is not a credit source.
	Included []string
	// Exhausted ids are in the session but must not be selected. The user
	// has no remaining token budget for them. Only a hook unavailable_models
	// list sets this.
	Exhausted []string
	// CreditsReported is true only when this call's hook payload sent
	// included_models or unavailable_models. A file on disk is not a credit report.
	CreditsReported bool
	// Usage evidence is separate from operator allowlists and hook marks.
	Usage         *quota.Snapshot
	QuotaRequired bool
	QuotaTime     time.Time
}

// WithUsageQuota attaches current provider evidence. A supplied snapshot,
// cached snapshot (even malformed), or required mode closes the quota gate.
// Missing evidence in legacy mode retains routing without claiming credit.
func (s SessionList) WithUsageQuota(harness string, supplied *quota.Snapshot) SessionList {
	s.QuotaTime = time.Now()
	s.QuotaRequired = os.Getenv("DOWNSHIFT_QUOTA_MODE") == "required"
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
	// An explicitly configured native export is fresher provider evidence than
	// the canonical cache. Invalid native input remains present and therefore
	// closes the gate instead of falling back to a stale cache.
	s.Usage, present = quota.LoadNative(harness)
	if !present {
		s.Usage, present = quota.Load(harness)
	}
	s.QuotaRequired = s.QuotaRequired || present
	return s
}

// QuotaStatus never converts inclusion or discovery into a balance claim.
func (s SessionList) QuotaStatus(harness, id string) quota.Status {
	if s.Usage == nil {
		return quota.Unknown
	}
	now := s.QuotaTime
	if now.IsZero() {
		now = time.Now()
	}
	return s.Usage.Evaluate(harness, id, now)
}

func (s SessionList) quotaAllows(harness, id string) bool {
	return !s.QuotaRequired || s.QuotaStatus(harness, id) == quota.Available
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
// An absent slice supplies no mark; a present slice is authoritative.
// Operator files never provide credit evidence.
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

// SessionFromHook returns the first non-nil allowlist from the hook payload.
// A nil slice means that field was absent. A non-nil slice is authoritative,
// even when empty, and the user file must not be consulted.
func SessionFromHook(lists ...*[]string) (SessionList, bool) {
	for _, list := range lists {
		if list != nil {
			return KnownSession(*list), true
		}
	}
	return SessionList{}, false
}

// DefaultSessionModelsPath is under the Downshift state dir (see internal/paths).
// The file is operator config. The binary does not embed a default allowlist.
func DefaultSessionModelsPath() string {
	p, err := paths.SessionModelsPath()
	if err != nil {
		return ""
	}
	return p
}

// LoadUserSession reads the harness key from the user file.
// DOWNSHIFT_SESSION_MODELS overrides the path (tests and operators).
// A missing file, invalid JSON, or a missing harness key is an unknown session.
func LoadUserSession(harness string) SessionList {
	return LoadUserSessionForID(harness, "")
}

// LoadUserSessionForID prefers an allowlist scoped to the exact harness
// session, then falls back to the harness-wide allowlist for compatibility.
func LoadUserSessionForID(harness, sessionID string) SessionList {
	path := os.Getenv("DOWNSHIFT_SESSION_MODELS")
	if path == "" {
		path = DefaultSessionModelsPath()
	}
	return LoadSessionFileForID(harness, sessionID, path)
}

// LoadSessionFile reads one harness allowlist from path.
func LoadSessionFile(harness, path string) SessionList {
	return LoadSessionFileForID(harness, "", path)
}

// LoadSessionFileForID checks sessions.<harness>.<session-id> first, then
// the legacy top-level <harness> list.
func LoadSessionFileForID(harness, sessionID, path string) SessionList {
	if harness == "" || path == "" {
		return UnknownSession()
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return UnknownSession()
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return UnknownSession()
	}
	if sessionID != "" {
		var byHarness map[string]map[string][]string
		if sessions, ok := raw["sessions"]; ok && json.Unmarshal(sessions, &byHarness) == nil {
			if ids, ok := byHarness[harness][sessionID]; ok && ids != nil {
				return KnownSession(ids)
			}
		}
	}
	msg, ok := raw[harness]
	if !ok {
		return UnknownSession()
	}
	var ids []string
	if err := json.Unmarshal(msg, &ids); err != nil {
		return UnknownSession()
	}
	return KnownSession(ids)
}

// ResolveSession prefers a hook allowlist. When the payload has none, it
// reads the user file. Every harness uses this order.
func ResolveSession(harness string, hookLists ...*[]string) SessionList {
	return ResolveSessionForID(harness, "", hookLists...)
}

// ResolveSessionForID prefers the hook payload, a configured native export,
// models recovered by `downshift models discover`, then the operator file. The file is not a
// credit report and does not outrank a recovered session.
func ResolveSessionForID(harness, sessionID string, hookLists ...*[]string) SessionList {
	if session, ok := SessionFromHook(hookLists...); ok {
		return session
	}
	if session, present := LoadNativeSession(harness, time.Now()); present {
		return session
	}
	if session := LoadDiscoveredSession(harness, time.Now()); session.Known {
		return session
	}
	return LoadUserSessionForID(harness, sessionID)
}

// ExplicitUpshiftEnabled reports whether upshift may select catalog
// explicit_only ids. The default is off. DOWNSHIFT_EXPLICIT_UPSHIFT=1 or
// "explicit_upshift": true in session-models.json turns it on. Either source
// is enough. The flag does not replace a current explicit_only model.
func ExplicitUpshiftEnabled() bool {
	if os.Getenv("DOWNSHIFT_EXPLICIT_UPSHIFT") == "1" {
		return true
	}
	return explicitUpshiftFromFile()
}

func explicitUpshiftFromFile() bool {
	path := os.Getenv("DOWNSHIFT_SESSION_MODELS")
	if path == "" {
		path = DefaultSessionModelsPath()
	}
	if path == "" {
		return false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	var raw struct {
		ExplicitUpshift bool `json:"explicit_upshift"`
	}
	if json.Unmarshal(data, &raw) != nil {
		return false
	}
	return raw.ExplicitUpshift
}
