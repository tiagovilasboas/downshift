// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package headroom

import (
	"math"
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/tiagovilasboas/downshift/internal/routing/contract"
)

const (
	claude contract.HarnessID = "claude-code"
	codex  contract.HarnessID = "codex"
	cursor contract.HarnessID = "cursor"
)

var (
	now = time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC)
	ttl = 10 * time.Minute
)

// reading is observed age before now, with the test TTL.
func reading(h contract.HarnessID, w contract.Window, pct float64, limited bool, age time.Duration) contract.Reading {
	return contract.Reading{Harness: h, Window: w, UsedPct: pct, RateLimited: limited, ObservedAt: now.Add(-age), TTL: ttl}
}

// live is a reading observed one minute ago, inside its TTL.
func live(h contract.HarnessID, w contract.Window, pct float64) contract.Reading {
	return reading(h, w, pct, false, time.Minute)
}

func TestAssessRules(t *testing.T) {
	cases := []struct {
		name      string
		providers []contract.HarnessID
		readings  []contract.Reading
		threshold float64
		want      []Verdict
	}{
		{
			name:      "worst window decides: week above short",
			providers: []contract.HarnessID{claude},
			readings:  []contract.Reading{live(claude, contract.WindowShort, 40), live(claude, contract.WindowWeek, 85)},
			threshold: DefaultCriticalThreshold,
			want:      []Verdict{{Harness: claude, Strain: StrainCritical, Window: contract.WindowWeek, UsedPct: 85}},
		},
		{
			name:      "worst window decides: short above week",
			providers: []contract.HarnessID{claude},
			readings:  []contract.Reading{live(claude, contract.WindowShort, 90), live(claude, contract.WindowWeek, 20)},
			threshold: DefaultCriticalThreshold,
			want:      []Verdict{{Harness: claude, Strain: StrainCritical, Window: contract.WindowShort, UsedPct: 90}},
		},
		{
			name:      "every window below threshold is ok at the worst one",
			providers: []contract.HarnessID{claude},
			readings:  []contract.Reading{live(claude, contract.WindowShort, 30), live(claude, contract.WindowWeek, 60)},
			threshold: DefaultCriticalThreshold,
			want:      []Verdict{{Harness: claude, Strain: StrainOK, Window: contract.WindowWeek, UsedPct: 60}},
		},
		{
			name:      "rate limited outranks any percentage",
			providers: []contract.HarnessID{claude},
			readings:  []contract.Reading{reading(claude, contract.WindowShort, 10, true, time.Minute), live(claude, contract.WindowWeek, 99)},
			threshold: DefaultCriticalThreshold,
			want:      []Verdict{{Harness: claude, Strain: StrainCritical, Window: contract.WindowShort, UsedPct: 10, RateLimited: true}},
		},
		{
			name:      "rate limited at zero percent is critical",
			providers: []contract.HarnessID{claude},
			readings:  []contract.Reading{reading(claude, contract.WindowShort, 0, true, time.Minute)},
			threshold: DefaultCriticalThreshold,
			want:      []Verdict{{Harness: claude, Strain: StrainCritical, Window: contract.WindowShort, RateLimited: true}},
		},
		{
			name:      "rate limited above any threshold is critical",
			providers: []contract.HarnessID{claude},
			readings:  []contract.Reading{reading(claude, contract.WindowShort, 10, true, time.Minute)},
			threshold: 100,
			want:      []Verdict{{Harness: claude, Strain: StrainCritical, Window: contract.WindowShort, UsedPct: 10, RateLimited: true}},
		},
		{
			name:      "two rate limited windows: window order, then percentage",
			providers: []contract.HarnessID{claude},
			readings:  []contract.Reading{reading(claude, contract.WindowShort, 90, true, time.Minute), reading(claude, contract.WindowWeek, 95, true, time.Minute)},
			threshold: DefaultCriticalThreshold,
			want:      []Verdict{{Harness: claude, Strain: StrainCritical, Window: contract.WindowShort, UsedPct: 90, RateLimited: true}},
		},
		{
			name:      "equal percentages: window order decides",
			providers: []contract.HarnessID{claude},
			readings:  []contract.Reading{live(claude, contract.WindowWeek, 85), live(claude, contract.WindowShort, 85)},
			threshold: DefaultCriticalThreshold,
			want:      []Verdict{{Harness: claude, Strain: StrainCritical, Window: contract.WindowShort, UsedPct: 85}},
		},
		{
			name:      "exact threshold is critical (>=)",
			providers: []contract.HarnessID{claude},
			readings:  []contract.Reading{live(claude, contract.WindowWeek, 80)},
			threshold: DefaultCriticalThreshold,
			want:      []Verdict{{Harness: claude, Strain: StrainCritical, Window: contract.WindowWeek, UsedPct: 80}},
		},
		{
			name:      "just below threshold is ok",
			providers: []contract.HarnessID{claude},
			readings:  []contract.Reading{live(claude, contract.WindowWeek, 79.9)},
			threshold: DefaultCriticalThreshold,
			want:      []Verdict{{Harness: claude, Strain: StrainOK, Window: contract.WindowWeek, UsedPct: 79.9}},
		},
		{
			name:      "overridden threshold lowers the bar",
			providers: []contract.HarnessID{claude},
			readings:  []contract.Reading{live(claude, contract.WindowWeek, 75)},
			threshold: 70,
			want:      []Verdict{{Harness: claude, Strain: StrainCritical, Window: contract.WindowWeek, UsedPct: 75}},
		},
		{
			name:      "overridden threshold raises the bar",
			providers: []contract.HarnessID{claude},
			readings:  []contract.Reading{live(claude, contract.WindowWeek, 85)},
			threshold: 90,
			want:      []Verdict{{Harness: claude, Strain: StrainOK, Window: contract.WindowWeek, UsedPct: 85}},
		},
		{
			name:      "stale reading is unknown and passes",
			providers: []contract.HarnessID{claude},
			readings:  []contract.Reading{reading(claude, contract.WindowWeek, 95, false, 11*time.Minute)},
			threshold: DefaultCriticalThreshold,
			want:      []Verdict{{Harness: claude}},
		},
		{
			name:      "stale window does not hold back a live one",
			providers: []contract.HarnessID{claude},
			readings:  []contract.Reading{live(claude, contract.WindowShort, 50), reading(claude, contract.WindowWeek, 99, false, 11*time.Minute)},
			threshold: DefaultCriticalThreshold,
			want:      []Verdict{{Harness: claude, Strain: StrainOK, Window: contract.WindowShort, UsedPct: 50}},
		},
		{
			name:      "reading observed after now is unknown",
			providers: []contract.HarnessID{claude},
			readings:  []contract.Reading{reading(claude, contract.WindowWeek, 99, false, -time.Minute)},
			threshold: DefaultCriticalThreshold,
			want:      []Verdict{{Harness: claude}},
		},
		{
			name:      "zero observation time is unknown",
			providers: []contract.HarnessID{claude},
			readings:  []contract.Reading{{Harness: claude, Window: contract.WindowWeek, UsedPct: 99, RateLimited: true}},
			threshold: DefaultCriticalThreshold,
			want:      []Verdict{{Harness: claude}},
		},
		{
			name:      "percentage out of range or NaN is unknown",
			providers: []contract.HarnessID{claude},
			readings:  []contract.Reading{live(claude, contract.WindowShort, 101), live(claude, contract.WindowWeek, math.NaN())},
			threshold: DefaultCriticalThreshold,
			want:      []Verdict{{Harness: claude}},
		},
		{
			name:      "unknown window name is ignored",
			providers: []contract.HarnessID{claude},
			readings:  []contract.Reading{live(claude, contract.Window("monthly"), 99)},
			threshold: DefaultCriticalThreshold,
			want:      []Verdict{{Harness: claude}},
		},
		{
			name:      "provider without readings is unknown, never critical",
			providers: []contract.HarnessID{codex},
			readings:  []contract.Reading{live(claude, contract.WindowWeek, 99)},
			threshold: 0,
			want:      []Verdict{{Harness: codex}},
		},
		{
			name:      "providers come back sorted, each judged on its own readings",
			providers: []contract.HarnessID{cursor, claude, codex},
			readings:  []contract.Reading{live(claude, contract.WindowWeek, 85), live(codex, contract.WindowShort, 20)},
			threshold: DefaultCriticalThreshold,
			want: []Verdict{
				{Harness: claude, Strain: StrainCritical, Window: contract.WindowWeek, UsedPct: 85},
				{Harness: codex, Strain: StrainOK, Window: contract.WindowShort, UsedPct: 20},
				{Harness: cursor},
			},
		},
		{
			name:      "a duplicate provider is reported once",
			providers: []contract.HarnessID{claude, claude},
			readings:  []contract.Reading{live(claude, contract.WindowWeek, 85)},
			threshold: DefaultCriticalThreshold,
			want:      []Verdict{{Harness: claude, Strain: StrainCritical, Window: contract.WindowWeek, UsedPct: 85}},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := Assess(c.providers, c.readings, now, c.threshold); !reflect.DeepEqual(got, c.want) {
				t.Fatalf("Assess() =\n%+v\nwant\n%+v", got, c.want)
			}
		})
	}
}

// TestAssessIsDeterministic checks every ordering of the readings and the
// providers: ties in strain and window must give the same verdict each time.
func TestAssessIsDeterministic(t *testing.T) {
	readings := []contract.Reading{
		reading(claude, contract.WindowShort, 90, true, time.Minute),
		reading(claude, contract.WindowWeek, 95, true, time.Minute),
		reading(claude, contract.WindowModel, 85, false, time.Minute),
		live(codex, contract.WindowWeek, 85),
		live(codex, contract.WindowShort, 85),
	}
	want := []Verdict{
		{Harness: claude, Strain: StrainCritical, Window: contract.WindowShort, UsedPct: 90, RateLimited: true},
		{Harness: codex, Strain: StrainCritical, Window: contract.WindowShort, UsedPct: 85},
		{Harness: cursor},
	}
	providers := []contract.HarnessID{cursor, claude, codex}

	permutations(readings, func(p []contract.Reading) {
		if got := Assess(providers, p, now, DefaultCriticalThreshold); !reflect.DeepEqual(got, want) {
			t.Fatalf("input order changed the verdict:\ngot  %+v\nwant %+v", got, want)
		}
	})
	if got := Assess([]contract.HarnessID{codex, claude, cursor}, readings, now, DefaultCriticalThreshold); !reflect.DeepEqual(got, want) {
		t.Fatalf("provider order changed the verdict:\ngot  %+v\nwant %+v", got, want)
	}
}

// permutations calls visit once per ordering of xs (Heap's algorithm). visit
// must not keep the slice.
func permutations[T any](xs []T, visit func([]T)) {
	a := slices.Clone(xs)
	var gen func(k int)
	gen = func(k int) {
		if k <= 1 {
			visit(a)
			return
		}
		gen(k - 1)
		for i := 0; i < k-1; i++ {
			if k%2 == 0 {
				a[i], a[k-1] = a[k-1], a[i]
			} else {
				a[0], a[k-1] = a[k-1], a[0]
			}
			gen(k - 1)
		}
	}
	gen(len(a))
}
