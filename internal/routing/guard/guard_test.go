// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package guard

import (
	"testing"

	"github.com/tiagovilasboas/downshift/internal/routing/contract"
)

const testHarness contract.HarnessID = "test-harness"

// fakeCatalog mirrors the real catalog: explicit_only rows are not indexed as
// exact automatic targets, so their ids are not in exact.
type fakeCatalog struct {
	exact    map[string]bool
	explicit map[string]bool
}

func (f fakeCatalog) IsExactID(harness, id string) bool {
	return harness == string(testHarness) && f.exact[id]
}

func (f fakeCatalog) IsExplicitOnly(harness, id string) bool {
	return harness == string(testHarness) && f.explicit[id]
}

var testCatalog = fakeCatalog{
	exact:    map[string]bool{"small-model": true, "mid-model": true, "frontier-model": true},
	explicit: map[string]bool{"explicit-model": true},
}

// base is a confident, risk-free move from frontier down to a known small model.
var base = Input{
	Harness:   testHarness,
	Confident: true,
	Requested: contract.TierFrontier,
	Target:    contract.TierSmall,
	TargetID:  "small-model",
}

func with(mod func(*Input)) Input {
	in := base
	mod(&in)
	return in
}

func TestCheck(t *testing.T) {
	cases := []struct {
		name string
		cat  Catalog
		in   Input
		want Result
	}{
		{
			name: "goes down to small: allowed",
			cat:  testCatalog,
			in:   base,
			want: Result{Allowed: true},
		},
		{
			name: "goes down one tier: allowed",
			cat:  testCatalog,
			in: with(func(in *Input) {
				in.Requested = contract.TierMid
				in.Target = contract.TierSmall
			}),
			want: Result{Allowed: true},
		},
		{
			name: "explicit_only target: refused",
			cat:  testCatalog,
			in: with(func(in *Input) {
				in.TargetID = "explicit-model"
			}),
			want: Result{Reason: contract.ReasonExplicitOnly},
		},
		{
			name: "foreign id: refused",
			cat:  testCatalog,
			in: with(func(in *Input) {
				in.TargetID = "foreign-model"
			}),
			want: Result{Reason: contract.ReasonForeignID},
		},
		{
			name: "empty target id: refused as foreign",
			cat:  testCatalog,
			in: with(func(in *Input) {
				in.TargetID = ""
			}),
			want: Result{Reason: contract.ReasonForeignID},
		},
		{
			name: "doubt: unconfident keeps the requested model",
			cat:  testCatalog,
			in: with(func(in *Input) {
				in.Confident = false
			}),
			want: Result{},
		},
		{
			name: "doubt: unknown requested tier keeps",
			cat:  testCatalog,
			in: with(func(in *Input) {
				in.Requested = contract.TierUnknown
			}),
			want: Result{},
		},
		{
			name: "doubt: unknown target tier keeps",
			cat:  testCatalog,
			in: with(func(in *Input) {
				in.Target = contract.TierUnknown
			}),
			want: Result{},
		},
		{
			name: "doubt: no catalog to confirm the target keeps",
			cat:  nil,
			in:   base,
			want: Result{},
		},
		{
			name: "never widen: target above the requested tier is refused",
			cat:  testCatalog,
			in: with(func(in *Input) {
				in.Requested = contract.TierSmall
				in.Target = contract.TierFrontier
				in.TargetID = "frontier-model"
			}),
			want: Result{Reason: contract.ReasonNeverWiden},
		},
		{
			name: "same tier has nothing to move",
			cat:  testCatalog,
			in: with(func(in *Input) {
				in.Requested = contract.TierMid
				in.Target = contract.TierMid
				in.TargetID = "mid-model"
			}),
			want: Result{},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := Check(tc.cat, tc.in); got != tc.want {
				t.Fatalf("Check() = %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestRankFollowsContractOrder(t *testing.T) {
	if rank(contract.TierSmall) >= rank(contract.TierMid) || rank(contract.TierMid) >= rank(contract.TierFrontier) {
		t.Fatal("rank must order small < mid < frontier")
	}
	if rank(contract.TierUnknown) != -1 {
		t.Fatalf("rank(unknown) = %d, want -1", rank(contract.TierUnknown))
	}
}
