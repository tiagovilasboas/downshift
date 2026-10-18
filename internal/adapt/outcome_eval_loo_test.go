// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package adapt

import (
	"math"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/core"
)

func testCostModel() CostModel {
	return CostModel{
		Prices:       [3]TierPrice{{InputM: 1, OutputM: 2}, {InputM: 3, OutputM: 6}, {InputM: 10, OutputM: 20}},
		RefInputTok:  1_000_000,
		RefOutputTok: 500_000,
	}
}

func TestCostModelAttemptAndValidate(t *testing.T) {
	cm := testCostModel()
	if got := cm.Attempt(TierSmall); math.Abs(got-2) > 1e-9 {
		t.Fatalf("small attempt = %v, want 2", got)
	}
	if got := cm.Attempt(TierFrontier); math.Abs(got-20) > 1e-9 {
		t.Fatalf("frontier attempt = %v, want 20", got)
	}
	if err := cm.Validate(); err != nil {
		t.Fatalf("valid model rejected: %v", err)
	}
	bad := cm
	bad.Prices[1] = TierPrice{}
	if bad.Validate() == nil {
		t.Fatal("unpriced tier must be rejected")
	}
	bad = cm
	bad.RefInputTok, bad.RefOutputTok = 0, 0
	if bad.Validate() == nil {
		t.Fatal("empty reference request must be rejected")
	}
}

func TestScoreAttemptsEscalatesUntilPass(t *testing.T) {
	by := tierResults{
		core.TierSmall:    {"a": false, "b": true, "c": false},
		core.TierMid:      {"a": false, "b": true, "c": false},
		core.TierFrontier: {"a": true, "b": true, "c": false},
	}
	cm := testCostModel()
	var row CostRow
	for _, id := range []string{"a", "b", "c"} {
		if err := scoreAttempts(&row, id, TierSmall, by, cm); err != nil {
			t.Fatal(err)
		}
	}
	// a: small, mid, frontier (pass). b: small (pass). c: small, mid, frontier (never passes).
	if row.Total != 3 || row.Completed != 2 || row.FirstPass != 1 || row.Attempts != 7 {
		t.Fatalf("row = %+v", row)
	}
	want := 2*cm.Attempt(TierSmall) + 2*cm.Attempt(TierMid) + 2*cm.Attempt(TierFrontier) + cm.Attempt(TierSmall)
	if math.Abs(row.Cost-want) > 1e-9 {
		t.Fatalf("cost = %v, want %v", row.Cost, want)
	}
	if got := row.PerCompleted(); math.Abs(got-want/2) > 1e-9 {
		t.Fatalf("per completed = %v", got)
	}
}

func TestScoreAttemptsMissingRunData(t *testing.T) {
	by := tierResults{core.TierSmall: {}, core.TierMid: {}, core.TierFrontier: {}}
	var row CostRow
	if err := scoreAttempts(&row, "x", TierSmall, by, testCostModel()); err == nil {
		t.Fatal("missing run data must be an error")
	}
}

func TestPerCompletedZeroWhenNothingCompleted(t *testing.T) {
	if (CostRow{Cost: 5}).PerCompleted() != 0 {
		t.Fatal("PerCompleted must be 0 when nothing completed")
	}
}

func TestLooMemoryExcludesSkippedTask(t *testing.T) {
	shapes := []taskShape{
		{id: "a", key: "K", events: []FeedbackEvent{{Shape: "K", SelectedTier: TierSmall, Success: true}}},
		{id: "b", key: "K", events: []FeedbackEvent{{Shape: "K", SelectedTier: TierMid, Success: true}}},
	}
	mem := looMemory(shapes, 0)
	rec := mem.Shapes["K"]
	if len(rec.Hits) != 1 || rec.Hits[0] != "MID" {
		t.Fatalf("skipping task a must leave only b's hit, got %+v", rec)
	}
}

func TestFailurePenaltyAppliesPerFailedAttempt(t *testing.T) {
	by := tierResults{
		core.TierSmall:    {"a": false},
		core.TierMid:      {"a": false},
		core.TierFrontier: {"a": true},
	}
	cm := testCostModel()
	cm.FailurePenalty = 100
	var row CostRow
	if err := scoreAttempts(&row, "a", TierSmall, by, cm); err != nil {
		t.Fatal(err)
	}
	want := cm.Attempt(TierSmall) + cm.Attempt(TierMid) + cm.Attempt(TierFrontier) + 200
	if math.Abs(row.Cost-want) > 1e-9 {
		t.Fatalf("cost = %v, want %v", row.Cost, want)
	}
	cm.FailurePenalty = -1
	if cm.Validate() == nil {
		t.Fatal("negative penalty must be rejected")
	}
}
