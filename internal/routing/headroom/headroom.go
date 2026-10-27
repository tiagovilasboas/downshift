// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

// Package headroom turns quota readings into a strain per provider: how close a
// provider is to its limit at one instant. It is pure. The caller passes the
// readings and the instant; the package does no I/O, reads no config and reads
// no clock.
//
// Rules (docs/design/routing-v2.md section 2, "Headroom"; clean-room behaviour
// of the REA orchestrator, sections 2 and 5):
//   - A window's strain is +Inf when it is rate limited, else its used
//     percentage. Rate limited outranks any percentage.
//   - A provider's verdict comes from its worst usable window. A tie in strain
//     goes to the window declared first in contract.Windows(), then to the
//     higher percentage. The result never depends on the input order.
//   - A reading that is not Fresh at the instant, or names no valid window, is
//     unknown. It counts for nothing: it passes every gate, and the other
//     windows still decide. A provider with no usable reading is unknown, never
//     critical.
//   - Critical means rate limited, or used percentage >= threshold (REA:
//     fitness >= T).
//
// Divergence from the REA: the REA keeps no TTL and trusts its last push
// (section 5, "Staleness"). Here a reading past its TTL is unknown.
//
// Downshift by Tiago Vilas Boas
// https://github.com/tiagovilasboas/downshift
package headroom

import (
	"math"
	"slices"
	"time"

	"github.com/tiagovilasboas/downshift/internal/routing/contract"
)

// DefaultCriticalThreshold is the used percentage at which a provider is
// critical when the policy sets no override (REA DEFAULT_CRITICAL_THRESHOLD).
const DefaultCriticalThreshold = 80

// Strain is the headroom level of one provider. The zero value is unknown.
type Strain string

const (
	StrainUnknown  Strain = ""         // no usable reading: passes every gate, never critical
	StrainOK       Strain = "ok"       // the worst usable window is below the threshold
	StrainCritical Strain = "critical" // rate limited, or the worst usable window is at or past the threshold
)

// Verdict is the strain of one provider and the window that decided it. Window,
// UsedPct and RateLimited map to the headroom fields of contract.Note. A Verdict
// with StrainUnknown has no window and no percentage.
type Verdict struct {
	Harness     contract.HarnessID `json:"harness"`
	Strain      Strain             `json:"strain"`
	Window      contract.Window    `json:"window,omitempty"`
	UsedPct     float64            `json:"used_pct,omitempty"`
	RateLimited bool               `json:"rate_limited,omitempty"`
}

// AllCritical is the O3 condition (mvp-plan section 2): every provider in
// verdicts is critical. The decision then warns and keeps the requested model;
// it never blocks and never forces the cheapest model. An unknown or ok
// provider is not critical, so one provider with usable headroom leaves routing
// open. No verdicts is not all critical.
func AllCritical(verdicts []Verdict) bool {
	if len(verdicts) == 0 {
		return false
	}
	for _, v := range verdicts {
		if v.Strain != StrainCritical {
			return false
		}
	}
	return true
}

// Assess returns one Verdict per distinct provider in providers, sorted by
// harness id. Only readings whose harness is in providers and that are Fresh at
// now count, in any order. threshold is compared with >=; the policy validates
// its range at load, and DefaultCriticalThreshold is the value when it is not
// overridden.
func Assess(providers []contract.HarnessID, readings []contract.Reading, now time.Time, threshold float64) []Verdict {
	ids := slices.Clone(providers)
	slices.Sort(ids)
	ids = slices.Compact(ids)
	out := make([]Verdict, 0, len(ids))
	for _, id := range ids {
		out = append(out, assessProvider(id, readings, now, threshold))
	}
	return out
}

// assessProvider picks the deciding reading of one provider and classifies it.
func assessProvider(id contract.HarnessID, readings []contract.Reading, now time.Time, threshold float64) Verdict {
	var worst contract.Reading
	found := false
	for _, r := range readings {
		if r.Harness != id || !r.Window.Valid() || !r.Fresh(now) {
			continue
		}
		if !found || outranks(r, worst) {
			worst, found = r, true
		}
	}
	if !found {
		return Verdict{Harness: id}
	}
	level := StrainOK
	if worst.RateLimited || worst.UsedPct >= threshold {
		level = StrainCritical
	}
	return Verdict{
		Harness:     id,
		Strain:      level,
		Window:      worst.Window,
		UsedPct:     worst.UsedPct,
		RateLimited: worst.RateLimited,
	}
}

// outranks reports whether reading a decides over reading b: higher strain
// first, then the earlier window in contract.Windows(), then the higher
// percentage. Every key is a verdict field, so readings that tie on all of
// them produce the same Verdict whatever the input order.
func outranks(a, b contract.Reading) bool {
	if sa, sb := strain(a), strain(b); sa != sb {
		return sa > sb
	}
	if ra, rb := windowRank(a.Window), windowRank(b.Window); ra != rb {
		return ra < rb
	}
	return a.UsedPct > b.UsedPct
}

// strain orders readings. A rate-limited window sits above every percentage.
func strain(r contract.Reading) float64 {
	if r.RateLimited {
		return math.Inf(1)
	}
	return r.UsedPct
}

// windowRank is the declaration order of w; callers pass valid windows only.
func windowRank(w contract.Window) int {
	return slices.Index(contract.Windows(), w)
}
