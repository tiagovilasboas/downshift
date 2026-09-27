// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: BUSL-1.1
// Commercial use requires a licence — see LICENSE for terms.

package core

import (
	"encoding/json"
	"os"
	"path/filepath"
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
	if !s.Known || id == "" {
		return false
	}
	for _, candidate := range s.IDs {
		if candidate == id {
			return true
		}
	}
	return false
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

// DefaultSessionModelsPath is ~/.harness-downshift/session-models.json.
// The file is operator config. The binary does not embed a default allowlist.
func DefaultSessionModelsPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".harness-downshift", "session-models.json")
}

// LoadUserSession reads the harness key from the user file.
// DOWNSHIFT_SESSION_MODELS overrides the path (tests and operators).
// A missing file, invalid JSON, or a missing harness key is an unknown session.
func LoadUserSession(harness string) SessionList {
	path := os.Getenv("DOWNSHIFT_SESSION_MODELS")
	if path == "" {
		path = DefaultSessionModelsPath()
	}
	return LoadSessionFile(harness, path)
}

// LoadSessionFile reads one harness allowlist from path.
func LoadSessionFile(harness, path string) SessionList {
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
	if session, ok := SessionFromHook(hookLists...); ok {
		return session
	}
	return LoadUserSession(harness)
}
