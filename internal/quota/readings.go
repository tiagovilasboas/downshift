// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package quota

import (
	"time"

	"github.com/tiagovilasboas/downshift/internal/routing/contract"
)

// nativeWindowRoles names the headroom window that each native window reports.
// A window a source does not name here yields no reading, so headroom treats it
// as unknown. Claude Code's statusline reports the five-hour and seven-day limits.
var nativeWindowRoles = map[string]map[string]contract.Window{
	"claude-statusline": {
		"five_hour": contract.WindowShort,
		"seven_day": contract.WindowWeek,
	},
}

// Readings turns a snapshot into headroom readings, one per named window that
// reports a percentage. Each reading is observed at the snapshot's observation
// time and lives for the snapshot's own lifetime, capped by quotaTTL, so headroom
// treats it as unknown once it is past its TTL. A missing window or an absent
// percentage produces no reading. The statusline carries no rate-limited flag,
// so a window at 100% is critical through its percentage alone.
func Readings(s Snapshot, quotaTTL time.Duration) []contract.Reading {
	roles := nativeWindowRoles[s.Source]
	ttl := s.ExpiresAt.Sub(s.ObservedAt)
	if quotaTTL > 0 && quotaTTL < ttl {
		ttl = quotaTTL
	}
	var out []contract.Reading
	for _, w := range s.Windows {
		role, named := roles[w.ID]
		if !named || w.UsedPercent == nil {
			continue
		}
		out = append(out, contract.Reading{
			Harness:    contract.HarnessID(s.Harness),
			Window:     role,
			UsedPct:    *w.UsedPercent,
			ObservedAt: s.ObservedAt,
			TTL:        ttl,
		})
	}
	return out
}
