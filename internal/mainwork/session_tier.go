// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package mainwork

import (
	"sort"

	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

// sessionFrontier reads the event log once and reports, per "harness/session",
// whether the session's latest decision ran under a frontier parent model.
// Nothing is read when no record carries a session. Fail-open: an unreadable
// log or a missing catalog gives an empty map.
func sessionFrontier(recs []Record) map[string]bool {
	need := false
	for _, r := range recs {
		if r.Session != "" {
			need = true
			break
		}
	}
	if !need || telemetry.DirectionCatalog == nil {
		return nil
	}
	events, err := telemetry.ReadEvents()
	if err != nil {
		return nil
	}
	return frontierSessions(events, telemetry.DirectionCatalog)
}

func frontierSessions(events []telemetry.Event, cat telemetry.ModelCatalog) map[string]bool {
	latest := map[string]telemetry.Event{}
	sort.SliceStable(events, func(i, j int) bool { return events[i].Timestamp < events[j].Timestamp })
	for _, ev := range events {
		if ev.SessionID != "" && ev.ParentModel != "" {
			latest[ev.Harness+"/"+ev.SessionID] = ev
		}
	}
	out := map[string]bool{}
	for key, ev := range latest {
		if _, tier, ok := cat.Lookup(ev.Harness, ev.ParentModel); ok && tier == telemetry.TierRankFrontier {
			out[key] = true
		}
	}
	return out
}
