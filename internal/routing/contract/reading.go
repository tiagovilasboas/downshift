// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package contract

import (
	"math"
	"time"
)

// Window labels a quota window (REA section 5: the short window, the weekly
// window and a model-scoped window). The zero value is unknown.
type Window string

const (
	WindowUnknown Window = ""
	WindowShort   Window = "short"
	WindowWeek    Window = "week"
	WindowModel   Window = "model"
)

// Windows lists every non-empty Window.
func Windows() []Window { return []Window{WindowShort, WindowWeek, WindowModel} }

// Valid reports whether w is one of Windows.
func (w Window) Valid() bool { return member(w, Windows()) }

// Reading is one provider window's headroom at a moment (REA section 5;
// design section 2 headroom bullet). The zero value is an explicit unknown:
// no observation time. routing/headroom reduces readings to a strain (worst
// window; rate limited outranks any percentage); that logic is not here.
//
// Divergence from the REA: a reading has a TTL, and one past it is unknown
// (missing data allows, so a stale reading can never hold work back).
type Reading struct {
	Harness     HarnessID
	Window      Window
	UsedPct     float64 // 0..100
	RateLimited bool
	ObservedAt  time.Time
	TTL         time.Duration
}

// Fresh reports whether r is a usable observation at now: observed, with a
// positive TTL, not observed after now, not older than its TTL, and with a
// percentage in 0..100. Anything else is unknown. This is the definition of
// "unknown" shared by every consumer, not a routing decision.
func (r Reading) Fresh(now time.Time) bool {
	if r.ObservedAt.IsZero() || r.TTL <= 0 || r.ObservedAt.After(now) {
		return false
	}
	if now.Sub(r.ObservedAt) > r.TTL {
		return false
	}
	return !math.IsNaN(r.UsedPct) && r.UsedPct >= 0 && r.UsedPct <= 100
}
