// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package core

import (
	"encoding/json"
	"os"
	"time"

	"github.com/tiagovilasboas/downshift/internal/paths"
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
	// Included is this harness's credit set. It comes from
	// quota.<harness>.included, or from a hook included_models list that
	// replaces the file for that call. Empty means this harness reported no
	// credit set: ranking stays the session order and no credit denial is
	// invented. A non-empty list is closed.
	Included []string
	// Exhausted ids are in the session but must not be selected. The user
	// has no remaining token budget for them. A hook unavailable_models
	// list replaces the file for that call.
	Exhausted []string
	// CreditsReported is true when this call's hook payload sent
	// included_models or unavailable_models. File quota still fills Included
	// and Exhausted. Selection treats a non-empty Included as the closed set
	// either way.
	CreditsReported bool
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
// A nil slice leaves the file value in place. A non-nil slice is authoritative.
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
				return withFileQuota(KnownSession(ids), harness, raw)
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
	return withFileQuota(KnownSession(ids), harness, raw)
}

// withFileQuota reads the optional quota.<harness> object.
// included: models that still have token budget for this harness.
// exhausted: models the user cannot run until the budget resets.
// A hook non-nil included_models or unavailable_models list replaces these
// slices for that call only (see WithHookQuota). The selector does not add
// ids the file omitted.
func withFileQuota(session SessionList, harness string, raw map[string]json.RawMessage) SessionList {
	msg, ok := raw["quota"]
	if !ok {
		return session
	}
	var quota map[string]struct {
		Included  []string `json:"included"`
		Exhausted []string `json:"exhausted"`
	}
	if json.Unmarshal(msg, &quota) != nil {
		return session
	}
	q, ok := quota[harness]
	if !ok {
		return session
	}
	if q.Included != nil {
		session.Included = append([]string(nil), q.Included...)
	}
	if q.Exhausted != nil {
		session.Exhausted = append([]string(nil), q.Exhausted...)
	}
	return session
}

// ResolveSession prefers a hook allowlist. When the payload has none, it
// reads the user file. Every harness uses this order.
func ResolveSession(harness string, hookLists ...*[]string) SessionList {
	return ResolveSessionForID(harness, "", hookLists...)
}

// ResolveSessionForID prefers a hook-provided allowlist, then the models the
// harness reported to `downshift models discover`, then the operator file.
// quota.<harness> from the operator file still applies to a hook or recovered
// list. A hook included_models or unavailable_models array replaces that
// quota for the call (see WithHookQuota).
func ResolveSessionForID(harness, sessionID string, hookLists ...*[]string) SessionList {
	if session, ok := SessionFromHook(hookLists...); ok {
		return applyFileQuota(session, harness)
	}
	if session := LoadDiscoveredSession(harness, time.Now()); session.Known {
		return applyFileQuota(session, harness)
	}
	return LoadUserSessionForID(harness, sessionID)
}

// applyFileQuota copies quota.<harness> onto a session whose ids came from
// the hook or from discovery. A missing file leaves the session unchanged.
func applyFileQuota(session SessionList, harness string) SessionList {
	if !session.Known || harness == "" {
		return session
	}
	path := os.Getenv("DOWNSHIFT_SESSION_MODELS")
	if path == "" {
		path = DefaultSessionModelsPath()
	}
	if path == "" {
		return session
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return session
	}
	var raw map[string]json.RawMessage
	if json.Unmarshal(data, &raw) != nil {
		return session
	}
	return withFileQuota(session, harness, raw)
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
