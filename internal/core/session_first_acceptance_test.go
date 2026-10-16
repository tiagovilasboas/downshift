// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package core_test

// harness-downshift by Tiago de Carvalho Vilas Boas
// https://github.com/tiagovilasboas/downshift

import (
	"encoding/json"
	"github.com/tiagovilasboas/downshift/internal/core"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestSessionFirstAcceptance_NewOrderedModelsAcrossHarnesses(t *testing.T) {
	for _, h := range core.AllHarnessCaps() {
		t.Run(h.Harness, func(t *testing.T) {
			session := core.KnownSession([]string{"qa-new-small", "qa-new-mid", "qa-new-frontier"})
			d := core.RouteForSession("rename the userId variable to userIdentifier", h.Harness, "qa-new-frontier", cat, session)
			if d.CurrentModel.ID != "qa-new-frontier" || d.CurrentModel.Tier != core.TierFrontier {
				t.Errorf("session baseline not resolved: %+v", d.CurrentModel)
			}
			if d.Tier != core.TierSmall || d.Model.ID != "qa-new-small" {
				t.Errorf("session route not selected: %+v", d)
			}
			if d.CurrentModel.InputM != 0 || d.CurrentModel.OutputM != 0 || d.Model.InputM != 0 || d.Model.OutputM != 0 || d.Savings != 0 {
				t.Errorf("unknown session prices invented: %+v", d)
			}
			foreign := core.RouteForSession("rename el campo que contiene las contraseñas de clientes", h.Harness, "qa-new-frontier", cat, session)
			if !foreign.UnsupportedLanguage || foreign.Confident || checkWrites(foreign) {
				t.Errorf("new baseline bypasses language guard: %+v", foreign)
			}
		})
	}
}

func TestSessionFirstAcceptance_MetadataBeatsCatalog(t *testing.T) {
	for _, h := range core.AllHarnessCaps() {
		current := catID(h.Harness, core.TierSmall)
		s := core.KnownSession([]string{current})
		// The active provider reports this known ID at a different tier. Its
		// explicit metadata must be resolved before comparing current and target.
		s.Metadata = []core.SessionModel{{ID: current, Harness: h.Harness, Tier: "frontier"}}
		r := core.ResolveSessionModels(h.Harness, s, cat)
		got, ok := r.LookupByID(h.Harness, current)
		if !ok || got.Tier != core.TierFrontier {
			t.Errorf("%s session metadata lost to catalog: %+v ok=%v", h.Harness, got, ok)
		}
	}
}

func TestSessionFirstAcceptance_NativeUnorderedNeverInventsTier(t *testing.T) {
	for _, h := range core.AllHarnessCaps() {
		s := core.KnownSession([]string{"qa-new-small", "qa-new-mid", "qa-new-frontier"})
		s.NativeAvailability = true
		r := core.ResolveSessionModels(h.Harness, s, cat)
		for _, id := range s.IDs {
			if m, ok := r.LookupByID(h.Harness, id); ok {
				t.Errorf("%s unordered unknown ID assigned tier: %+v", h.Harness, m)
			}
		}
	}
}

func TestSessionFirstAcceptance_InvalidNativeTierCannotBecomePositional(t *testing.T) {
	for _, h := range core.AllHarnessCaps() {
		s := core.KnownSession([]string{"qa-new-small", "qa-invalid", "qa-new-frontier"})
		s.Metadata = []core.SessionModel{{ID: "qa-invalid", Tier: "super-ultra", Harness: h.Harness}}
		r := core.ResolveSessionModels(h.Harness, s, cat)
		if m, ok := r.LookupByID(h.Harness, "qa-invalid"); ok {
			t.Errorf("%s invalid explicit metadata reopened positional fallback: %+v", h.Harness, m)
		}
	}
}

func TestSessionFirstAcceptance_DiscoveredMetadataRoundtrip(t *testing.T) {
	t.Setenv(core.EnvDiscovery, "")
	now := time.Now()
	for _, h := range core.AllHarnessCaps() {
		t.Run(h.Harness, func(t *testing.T) {
			metadata := []core.SessionModel{{ID: "qa-opaque", Tier: "frontier", Harness: h.Harness, Efforts: []string{"low", "high"}}}
			payload := map[string]any{"harnesses": map[string]any{h.Harness: map[string]any{"fetched_at": now, "ordered": []string{"qa-opaque"}, "model_metadata": metadata}}}
			raw, err := json.Marshal(payload)
			if err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(t.TempDir(), "discovered.json")
			if err := os.WriteFile(path, raw, 0600); err != nil {
				t.Fatal(err)
			}
			t.Setenv(core.EnvDiscovered, path)
			session := core.LoadDiscoveredSession(h.Harness, now)
			if !reflect.DeepEqual(session.Metadata, metadata) {
				t.Fatalf("metadata did not survive JSON cache: %+v", session)
			}
			r := core.ResolveSessionModels(h.Harness, session, cat)
			model, ok := r.LookupByID(h.Harness, "qa-opaque")
			if !ok || model.Tier != core.TierFrontier || model.InputM != 0 || model.OutputM != 0 {
				t.Fatalf("cached native capability lost: %+v ok=%v", model, ok)
			}
		})
	}
}

func TestSessionFirstAcceptance_MalformedMetadataIsRejected(t *testing.T) {
	negative, nan, inf := -1.0, math.NaN(), math.Inf(1)
	for _, h := range core.AllHarnessCaps() {
		id := catID(h.Harness, core.TierMid)
		cases := map[string][]core.SessionModel{
			"duplicate":       {{ID: id, Tier: "mid"}, {ID: id, Tier: "frontier"}},
			"foreign harness": {{ID: id, Tier: "mid", Harness: "other-harness"}},
			"negative price":  {{ID: id, Tier: "mid", InputM: &negative}},
			"nan price":       {{ID: id, Tier: "mid", OutputM: &nan}},
			"infinite price":  {{ID: id, Tier: "mid", OutputM: &inf}},
			"invalid effort":  {{ID: id, Tier: "mid", Efforts: []string{"high\nunsafe"}}},
		}
		for name, metadata := range cases {
			t.Run(h.Harness+"/"+name, func(t *testing.T) {
				s := core.KnownSession([]string{"qa-new-small", id, "qa-new-frontier"})
				s.Metadata = metadata
				r := core.ResolveSessionModels(h.Harness, s, cat)
				if model, ok := r.LookupByID(h.Harness, id); ok {
					t.Errorf("malformed source reopened positional/catalog fallback: %+v", model)
				}
			})
		}
	}
}

func TestSessionFirstAcceptance_RejectedMetadataCannotBeWritten(t *testing.T) {
	for _, h := range core.AllHarnessCaps() {
		s := core.KnownSession([]string{"qa-new-small", "qa-invalid", "qa-new-frontier"})
		s.Metadata = []core.SessionModel{{ID: "qa-new-small", Tier: "small"}, {ID: "qa-invalid", Tier: "bad tier"}, {ID: "qa-new-frontier", Tier: "frontier"}}
		r := core.ResolveSessionModels(h.Harness, s, cat)
		d := core.RouteForSession("implement a feature endpoint", h.Harness, "", cat, s)
		p := d.PlanForSession(h.Caps, r, s)
		if p.Model.ID == "qa-invalid" {
			t.Errorf("%s rejected model reached plan: %+v", h.Harness, p)
		}
		if core.CanWriteSessionID(h.Harness, "qa-invalid", s, r) {
			t.Errorf("%s write gate accepts rejected metadata", h.Harness)
		}
	}
}

func TestSessionFirstAcceptance_FutureEffortDoesNotInventMapping(t *testing.T) {
	for _, h := range core.AllHarnessCaps() {
		s := core.KnownSession([]string{"qa-opaque"})
		s.Metadata = []core.SessionModel{{ID: "qa-opaque", Tier: "frontier", Efforts: []string{"future_ultra"}}}
		r := core.ResolveSessionModels(h.Harness, s, cat)
		if _, known := r.LookupByID(h.Harness, "qa-opaque"); !known {
			t.Errorf("%s valid future native effort rejected model", h.Harness)
		}
		if _, ok, _ := core.ConstrainEffort(r, h.Harness, "qa-opaque", core.EffortHigh); ok {
			t.Errorf("%s invented known effort mapping for future native token", h.Harness)
		}
	}
}
