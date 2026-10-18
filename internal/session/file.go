// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package session

import (
	"encoding/json"
	"os"
	"time"

	"github.com/tiagovilasboas/downshift/internal/paths"
)

// SessionFromHook returns the first non-nil allowlist from the hook payload.
// A nil slice means that field was absent. A non-nil slice is authoritative,
// even when empty, and the user file must not be consulted.
func SessionFromHook(lists ...*[]string) (SessionList, bool) {
	for index, list := range lists {
		if list != nil {
			session := KnownSession(*list)
			session.Unordered = index > 0
			return session, true
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
	path := sessionModelsEnv()
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
				return applyFileQuota(KnownSession(ids), raw, harness)
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
	return applyFileQuota(KnownSession(ids), raw, harness)
}

// harnessQuotaMarks is quota.<harness> inside session-models.json.
// A missing slice is not a mark. The harness allowlist is a different key.
type harnessQuotaMarks struct {
	Included  *[]string `json:"included"`
	Exhausted *[]string `json:"exhausted"`
}

// applyFileQuota copies quota.<harness> onto session from the document
// already unmarshaled by LoadSessionFileForID. Another harness's marks
// stay on that harness. An absent quota key leaves the allowlist as a
// known session with no invented credits.
func applyFileQuota(list SessionList, raw map[string]json.RawMessage, harness string) SessionList {
	msg, ok := raw["quota"]
	if !ok || !list.Known || harness == "" {
		return list
	}
	var byHarness map[string]json.RawMessage
	if json.Unmarshal(msg, &byHarness) != nil {
		return list
	}
	entry, ok := byHarness[harness]
	if !ok {
		return list
	}
	var marks harnessQuotaMarks
	if json.Unmarshal(entry, &marks) != nil || (marks.Included == nil && marks.Exhausted == nil) {
		return list
	}
	if marks.Included != nil {
		list.Included = append([]string(nil), (*marks.Included)...)
	}
	if marks.Exhausted != nil {
		list.Exhausted = append([]string(nil), (*marks.Exhausted)...)
	}
	list.CreditsReported = true
	return list
}

// ResolveSession prefers a hook allowlist. When the payload has none, it
// reads the user file. Every harness uses this order.
func ResolveSession(harness string, hookLists ...*[]string) SessionList {
	return ResolveSessionForID(harness, "", hookLists...)
}

// ResolveSessionForID prefers the hook payload, a configured native export,
// models recovered by `downshift models discover`, then the operator file.
// The allowlist is not a credit report and does not outrank a recovered
// session. quota.<harness> on that file is credit evidence for that harness.
func ResolveSessionForID(harness, sessionID string, hookLists ...*[]string) SessionList {
	if list, ok := SessionFromHook(hookLists...); ok {
		return list
	}
	if LoadNativeSessionHook != nil {
		if list, present := LoadNativeSessionHook(harness, time.Now()); present {
			return list
		}
	}
	if list := LoadDiscoveredSession(harness, time.Now()); list.Known {
		return list
	}
	return LoadUserSessionForID(harness, sessionID)
}

// ExplicitUpshiftEnabled reports whether upshift may select catalog
// explicit_only ids. The default is off. DOWNSHIFT_EXPLICIT_UPSHIFT=1 or
// "explicit_upshift": true in session-models.json turns it on. Either source
// is enough. The flag does not replace a current explicit_only model.
func ExplicitUpshiftEnabled() bool {
	if explicitUpshiftEnv() {
		return true
	}
	return explicitUpshiftFromFile()
}

func explicitUpshiftFromFile() bool {
	path := sessionModelsEnv()
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
