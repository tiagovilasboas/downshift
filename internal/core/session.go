// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package core

import (
	"time"

	"github.com/tiagovilasboas/downshift/internal/quotamodel"
	"github.com/tiagovilasboas/downshift/internal/session"
)

// SessionList is the allowlist a hook may write. The loader lives in
// internal/session; this alias keeps existing call sites compiling.
type SessionList = session.SessionList

// SessionModel is exact-id capability evidence carried on a SessionList.
type SessionModel = session.SessionModel

func init() {
	// The session package must not import core. Wire the env seam and the
	// quota hooks so loaders see the same snapshot and the same evidence
	// quota registers on this package.
	session.SetLookup(func(key string) string { return lookupEnv(key) })
	session.LoadQuotaHook = func(harness string) (*quotamodel.Snapshot, bool) {
		if LoadQuotaHook == nil {
			return nil, false
		}
		return LoadQuotaHook(harness)
	}
	session.LoadNativeQuotaHook = func(harness string) (*quotamodel.Snapshot, bool) {
		if LoadNativeQuotaHook == nil {
			return nil, false
		}
		return LoadNativeQuotaHook(harness)
	}
	session.LoadNativeSessionHook = LoadNativeSession
}

// KnownSession marks ids as the session allowlist, including a known-empty list.
func KnownSession(ids []string) SessionList { return session.KnownSession(ids) }

// UnknownSession is the fail-open list: no rewrite.
func UnknownSession() SessionList { return session.UnknownSession() }

// StaleQuotaHold reports whether a loaded snapshot for harness has expired but
// still holds model rewrites, and for how long it has been expired.
func StaleQuotaHold(harness string, now time.Time) (time.Duration, bool) {
	return session.StaleQuotaHold(harness, now)
}

// SessionFromHook returns the first non-nil allowlist from the hook payload.
func SessionFromHook(lists ...*[]string) (SessionList, bool) {
	return session.SessionFromHook(lists...)
}

// DefaultSessionModelsPath is under the Downshift state dir (see internal/paths).
func DefaultSessionModelsPath() string { return session.DefaultSessionModelsPath() }

// LoadUserSession reads the harness key from the user file.
func LoadUserSession(harness string) SessionList { return session.LoadUserSession(harness) }

// LoadUserSessionForID prefers an allowlist scoped to the exact harness session.
func LoadUserSessionForID(harness, sessionID string) SessionList {
	return session.LoadUserSessionForID(harness, sessionID)
}

// LoadSessionFile reads one harness allowlist from path.
func LoadSessionFile(harness, path string) SessionList {
	return session.LoadSessionFile(harness, path)
}

// LoadSessionFileForID checks sessions.<harness>.<session-id> first, then
// the legacy top-level <harness> list.
func LoadSessionFileForID(harness, sessionID, path string) SessionList {
	return session.LoadSessionFileForID(harness, sessionID, path)
}

// ResolveSession prefers a hook allowlist. When the payload has none, it
// reads the user file. Every harness uses this order.
func ResolveSession(harness string, hookLists ...*[]string) SessionList {
	return session.ResolveSession(harness, hookLists...)
}

// ResolveSessionForID prefers the hook payload, a configured native export,
// models recovered by `downshift models discover`, then the operator file.
func ResolveSessionForID(harness, sessionID string, hookLists ...*[]string) SessionList {
	return session.ResolveSessionForID(harness, sessionID, hookLists...)
}

// ExplicitUpshiftEnabled reports whether upshift may select catalog
// explicit_only ids. The default is off.
func ExplicitUpshiftEnabled() bool { return session.ExplicitUpshiftEnabled() }

// LoadDiscoveredSession reads the models `downshift models discover` found for
// harness. An absent, invalid, empty or expired entry is an unknown session.
func LoadDiscoveredSession(harness string, now time.Time) SessionList {
	return session.LoadDiscoveredSession(harness, now)
}
