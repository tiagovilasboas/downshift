// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package quota

import (
	"reflect"
	"testing"
	"time"

	"github.com/tiagovilasboas/downshift/internal/routing/contract"
	"github.com/tiagovilasboas/downshift/internal/routing/headroom"
)

func TestReadingsFeedHeadroomFromClaudeStatusline(t *testing.T) {
	now := time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC)
	claude := contract.HarnessID("claude-code")
	window := func(id string, used float64) Window {
		return Window{ID: id, Scope: "harness", UsedPercent: number(used), ResetsAt: now.Add(time.Hour)}
	}
	statusline := func(observed time.Duration, ttl time.Duration, windows ...Window) Snapshot {
		return Snapshot{Version: 1, Harness: "claude-code", Source: "claude-statusline", ObservedAt: now.Add(-observed), ExpiresAt: now.Add(-observed + ttl), Windows: windows}
	}
	cases := []struct {
		name         string
		snap         Snapshot
		at           time.Time
		quotaTTL     time.Duration
		wantReadings int
		want         []headroom.Verdict
	}{
		{
			name:         "fresh reading counts: short window decides",
			snap:         statusline(time.Minute, 5*time.Minute, window("five_hour", 85), window("seven_day", 20)),
			at:           now,
			quotaTTL:     5 * time.Minute,
			wantReadings: 2,
			want:         []headroom.Verdict{{Harness: claude, Strain: headroom.StrainCritical, Window: contract.WindowShort, UsedPct: 85}},
		},
		{
			name:         "fresh reading below threshold is ok at the worst window",
			snap:         statusline(time.Minute, 5*time.Minute, window("five_hour", 40), window("seven_day", 60)),
			at:           now,
			quotaTTL:     5 * time.Minute,
			wantReadings: 2,
			want:         []headroom.Verdict{{Harness: claude, Strain: headroom.StrainOK, Window: contract.WindowWeek, UsedPct: 60}},
		},
		{
			name:         "expired reading is unknown and passes",
			snap:         statusline(10*time.Minute, 5*time.Minute, window("five_hour", 95)),
			at:           now,
			quotaTTL:     5 * time.Minute,
			wantReadings: 1,
			want:         []headroom.Verdict{{Harness: claude}},
		},
		{
			name:         "snapshot past its own expiry is unknown and passes",
			snap:         statusline(6*time.Minute, 5*time.Minute, window("five_hour", 95)),
			at:           now,
			quotaTTL:     5 * time.Minute,
			wantReadings: 1,
			want:         []headroom.Verdict{{Harness: claude}},
		},
		{
			name:         "policy quota TTL caps a longer snapshot lifetime",
			snap:         statusline(3*time.Minute, 8*time.Minute, window("five_hour", 95)),
			at:           now.Add(3 * time.Minute),
			quotaTTL:     5 * time.Minute,
			wantReadings: 1,
			want:         []headroom.Verdict{{Harness: claude}},
		},
		{
			name:         "missing window is unknown, the other window still decides",
			snap:         statusline(time.Minute, 5*time.Minute, window("five_hour", 90)),
			at:           now,
			quotaTTL:     5 * time.Minute,
			wantReadings: 1,
			want:         []headroom.Verdict{{Harness: claude, Strain: headroom.StrainCritical, Window: contract.WindowShort, UsedPct: 90}},
		},
		{
			name:         "no windows at all is unknown and passes",
			snap:         statusline(time.Minute, 5*time.Minute),
			at:           now,
			quotaTTL:     5 * time.Minute,
			wantReadings: 0,
			want:         []headroom.Verdict{{Harness: claude}},
		},
		{
			name:         "window without a percentage is unknown and passes",
			snap:         statusline(time.Minute, 5*time.Minute, Window{ID: "five_hour", Scope: "harness", ResetsAt: now.Add(time.Hour)}),
			at:           now,
			quotaTTL:     5 * time.Minute,
			wantReadings: 0,
			want:         []headroom.Verdict{{Harness: claude}},
		},
		{
			name: "operator-normalized snapshot names no headroom window",
			snap: Snapshot{Version: 1, Harness: "claude-code", Source: "operator-normalized", ObservedAt: now.Add(-time.Minute), ExpiresAt: now.Add(4 * time.Minute),
				Windows: []Window{window("five_hour", 99)}},
			at:           now,
			quotaTTL:     5 * time.Minute,
			wantReadings: 0,
			want:         []headroom.Verdict{{Harness: claude}},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			readings := Readings(c.snap, c.quotaTTL)
			if len(readings) != c.wantReadings {
				t.Fatalf("Readings() = %d readings, want %d: %+v", len(readings), c.wantReadings, readings)
			}
			got := headroom.Assess([]contract.HarnessID{claude}, readings, c.at, headroom.DefaultCriticalThreshold)
			if !reflect.DeepEqual(got, c.want) {
				t.Fatalf("Assess() =\n%+v\nwant\n%+v", got, c.want)
			}
		})
	}
}
