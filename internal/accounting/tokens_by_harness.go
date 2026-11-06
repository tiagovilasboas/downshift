// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package accounting

import (
	"sort"
	"time"

	"github.com/tiagovilasboas/downshift/internal/execwrap"
	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

// Quota statuses of a harness row.
const (
	QuotaNative  = "native"  // a native snapshot exists
	QuotaUnknown = "unknown" // no native snapshot
)

// UnknownHarness is the harness name given to exec events that carry none.
const UnknownHarness = "unknown"

// QuotaUsed is the quota a harness has used, from its native snapshot only.
// The caller reads the snapshot; accounting only carries it.
type QuotaUsed struct {
	Status string `json:"status"`
	// FiveHourPct and WeekPct are nil when the window is not reported.
	FiveHourPct *float64 `json:"five_hour_pct,omitempty"`
	WeekPct     *float64 `json:"week_pct,omitempty"`
	// ObservedAt is when the snapshot was read; Stale says it is past its expiry.
	ObservedAt time.Time `json:"observed_at,omitempty"`
	Stale      bool      `json:"stale,omitempty"`
}

// HarnessTokens is one line of the report, never summed across columns or
// rows: tokens the Compactor kept out of the context, the routing tokens (the
// billable tokens that ran below the session model) and the quota used.
// Routing does not remove tokens; it runs them on a lower tier.
type HarnessTokens struct {
	Harness string `json:"harness"`
	// CompactorAvoided is saved bytes / execwrap.BytesPerToken: an estimate. It
	// is left out of this row's JSON: `report --json` prints the Compactor
	// tokens once, under compaction_by_harness.
	CompactorAvoided int64 `json:"-"`
	// RoutingObserved is false when no spawn of this harness could be placed
	// against the session model nor, as a fallback, the requested model; the
	// column then reads "unobserved", never 0. Routing carries the session
	// and the fallback figures apart: they are different baselines and are
	// never added.
	RoutingObserved bool                 `json:"routing_observed"`
	Routing         telemetry.RoutingRow `json:"routing"`
	Quota           QuotaUsed            `json:"quota_used"`
}

// TokensByHarness returns one HarnessTokens per harness that has a decision or
// an exec event in scope, sorted by name. quota maps a harness to its native
// snapshot reading; a harness missing there is QuotaUnknown. Pure.
func TokensByHarness(events []telemetry.Event, execEvents []execwrap.Event, scope Scope, quota map[string]QuotaUsed) []HarnessTokens {
	scoped := scopeTelemetry(events, scope)
	rows := map[string]*HarnessTokens{}
	row := func(h string) *HarnessTokens {
		if rows[h] == nil {
			q, ok := quota[h]
			if !ok || q.Status == "" {
				q = QuotaUsed{Status: QuotaUnknown}
			}
			rows[h] = &HarnessTokens{Harness: h, Quota: q}
		}
		return rows[h]
	}
	byHarness := map[string][]telemetry.Event{}
	for _, ev := range scoped {
		if ev.Harness != "" {
			byHarness[ev.Harness] = append(byHarness[ev.Harness], ev)
		}
	}
	for h, evs := range byHarness {
		if !hasDecision(evs) {
			continue
		}
		r := row(h)
		routing := telemetry.RoutingTokens(evs).ByHarness[h]
		r.Routing = routing
		r.RoutingObserved = routing.Observed() || routing.FallbackObserved()
	}
	saved := map[string]int64{}
	for _, ev := range scopeExec(execEvents, scope, time.Now()) {
		h := ev.Harness
		if h == "" {
			h = UnknownHarness
		}
		row(h)
		saved[h] += ev.NetSaved()
	}
	out := make([]HarnessTokens, 0, len(rows))
	for h, r := range rows {
		r.CompactorAvoided = saved[h] / execwrap.BytesPerToken
		out = append(out, *r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Harness < out[j].Harness })
	return out
}

func hasDecision(evs []telemetry.Event) bool {
	for _, ev := range evs {
		if telemetry.IsDecisionRow(ev) {
			return true
		}
	}
	return false
}
