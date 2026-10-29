// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package policy

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/core"
)

func writeFloor(t *testing.T, dir, body string) {
	t.Helper()
	p := filepath.Join(dir, filepath.FromSlash(FloorRelPath))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestFloorTakesHighestApplicableRule(t *testing.T) {
	p := &FloorPolicy{
		MinTier:        "small",
		MinTierByClass: map[string]string{"MEDIUM": "mid", "COMPLEX": "frontier"},
		MinTierByRisk:  map[string]string{"payment": "frontier"},
	}
	cases := []struct {
		class string
		risks []string
		want  core.Tier
	}{
		{"SIMPLE", nil, core.TierSmall},
		{"MEDIUM", nil, core.TierMid},
		{"COMPLEX", nil, core.TierFrontier},
		{"SIMPLE", []string{"payment"}, core.TierFrontier},
	}
	for _, c := range cases {
		if got, ok := p.Floor(c.class, c.risks); !ok || got != c.want {
			t.Errorf("Floor(%s,%v) = %v,%v want %v", c.class, c.risks, got, ok, c.want)
		}
	}
	if _, ok := (*FloorPolicy)(nil).Floor("COMPLEX", nil); ok {
		t.Error("a nil policy applies nothing")
	}
}

func TestFloorLoadRejectsInvalidFiles(t *testing.T) {
	bad := map[string]string{
		"not json":      `nope`,
		"wrong version": `{"version":2}`,
		"unknown tier":  `{"version":1,"min_tier":"huge"}`,
		"unknown class": `{"version":1,"min_tier_by_class":{"EASY":"mid"}}`,
		"unknown risk":  `{"version":1,"min_tier_by_risk":{"typo":"mid"}}`,
		"unknown field": `{"version":1,"min_tir":"mid"}`,
		"oversized":     `{"version":1,"pad":"` + strings.Repeat("x", FloorMaxBytes) + `"}`,
	}
	for name, body := range bad {
		dir := t.TempDir()
		writeFloor(t, dir, body)
		p, path, err := LoadNearestFloor(dir)
		if p != nil || err == nil || path == "" {
			t.Errorf("%s: want error with path, got %v %q %v", name, p, path, err)
		}
	}
}

func TestFloorAcceptsRoutingSection(t *testing.T) {
	dir := t.TempDir()
	writeFloor(t, dir, `{"version":1,"min_tier":"mid","routing":{"rules":[]}}`)
	p, _, err := LoadNearestFloor(dir)
	if err != nil {
		t.Fatalf("routing section refused the floor: %v", err)
	}
	if got, ok := p.Floor("SIMPLE", nil); !ok || got != core.TierMid {
		t.Fatalf("Floor = %v,%v; want mid,true", got, ok)
	}
}

func TestFloorApplyOnlyRaisesTier(t *testing.T) {
	dir := t.TempDir()
	writeFloor(t, dir, `{"version":1,"min_tier":"mid"}`)
	d := core.Route("rename the variable userId to accountId in auth.ts", "claude-code", "")
	if d.Tier != core.TierSmall {
		t.Skipf("fixture no longer routes small: %v", d.Tier)
	}
	ApplyFloor(&d, dir, "", nil)
	if d.Tier != core.TierMid || !d.Guards.PolicyFloor {
		t.Fatalf("floor must raise to mid, got %v floor %v", d.Tier, d.Guards.PolicyFloor)
	}
	other := t.TempDir()
	writeFloor(t, other, `{"version":1,"min_tier":"small"}`)
	ApplyFloor(&d, other, "", nil)
	if d.Tier != core.TierMid {
		t.Fatalf("floor must never lower the tier, got %v", d.Tier)
	}
}

func TestFloorApplyFailsOpenOnInvalidFile(t *testing.T) {
	dir := t.TempDir()
	writeFloor(t, dir, `nope`)
	d := core.Route("rename the variable userId to accountId in auth.ts", "claude-code", "")
	before := d.Tier
	ApplyFloor(&d, dir, "", nil)
	if d.Tier != before || d.Guards.PolicyFloor {
		t.Fatalf("invalid file must leave the decision untouched, got %v", d.Tier)
	}
}
