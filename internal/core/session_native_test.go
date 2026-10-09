// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package core

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tiagovilasboas/downshift/internal/quota"
)

func nativeFixture(now time.Time) map[string]any {
	return map[string]any{
		"observed_at": now.Add(-time.Second), "available_models_complete": true,
		"available_models":   []string{"strong", "unknown", "cheap", "middle"},
		"auto_bucket_models": []string{"cheap"}, "billing_cycle_end": now.Add(time.Hour).UnixMilli(),
		"plan_usage": map[string]any{"auto_percent_used": 60, "api_percent_used": 100},
	}
}

func writeNativeFixture(t *testing.T, payload map[string]any) string {
	t.Helper()
	b, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(t.TempDir(), "cursor-usage.json")
	if err := os.WriteFile(p, b, 0600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestNativeAvailabilityPrecedenceAndSharedObservation(t *testing.T) {
	now := time.Now().UTC()
	t.Setenv("DOWNSHIFT_CURSOR_NATIVE_FILE", writeNativeFixture(t, nativeFixture(now)))
	t.Setenv(EnvDiscovery, "")
	discovered := filepath.Join(t.TempDir(), "discovered.json")
	b, _ := json.Marshal(map[string]any{"harnesses": map[string]any{"cursor": map[string]any{"fetched_at": now, "ordered": []string{"from-cache"}}}})
	if err := os.WriteFile(discovered, b, 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv(EnvDiscovered, discovered)
	op := filepath.Join(t.TempDir(), "session-models.json")
	if err := os.WriteFile(op, []byte(`{"cursor":["from-operator"]}`), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("DOWNSHIFT_SESSION_MODELS", op)
	s := ResolveSession("cursor")
	if !s.Known || !s.NativeAvailability || len(s.IDs) != 4 || s.IDs[0] != "strong" {
		t.Fatalf("native list lost precedence or provider identity: %+v", s)
	}
	observed := s.Usage.ObservedAt
	// Rotation after resolution must not split availability from its quota.
	if err := os.WriteFile(os.Getenv("DOWNSHIFT_CURSOR_NATIVE_FILE"), []byte("malformed"), 0600); err != nil {
		t.Fatal(err)
	}
	s = s.WithUsageQuota("cursor", nil)
	if s.QuotaStatus("cursor", "cheap") != quota.Available || !s.Usage.ObservedAt.Equal(observed) {
		t.Fatal("availability and quota were not the same observation")
	}
	if fallback := ResolveSession("cursor"); fallback.Known {
		t.Fatalf("invalid configured native source reopened fallback: %+v", fallback)
	}
	hook := []string{"from-hook"}
	if s := ResolveSession("cursor", &hook); !s.Known || s.NativeAvailability || s.IDs[0] != "from-hook" {
		t.Fatalf("hook must beat even invalid native availability: %+v", s)
	}
}

func TestNativeAvailabilityRejectsInvalidAndExpiredInput(t *testing.T) {
	now := time.Now().UTC()
	for _, tc := range []struct {
		name string
		edit func(map[string]any)
	}{
		{"missing time", func(p map[string]any) { delete(p, "observed_at") }},
		{"future", func(p map[string]any) { p["observed_at"] = now.Add(time.Nanosecond) }},
		{"expired", func(p map[string]any) { p["observed_at"] = now.Add(-5 * time.Minute) }},
		{"incomplete", func(p map[string]any) { p["available_models_complete"] = false }},
		{"missing models", func(p map[string]any) { delete(p, "available_models") }},
		{"duplicate models", func(p map[string]any) { p["available_models"] = []string{"cheap", "cheap"} }},
		{"bad model", func(p map[string]any) { p["available_models"] = []string{"bad model"} }},
		{"invalid quota", func(p map[string]any) { p["plan_usage"] = map[string]any{"auto_percent_used": 101} }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := nativeFixture(now)
			tc.edit(p)
			t.Setenv("DOWNSHIFT_CURSOR_NATIVE_FILE", writeNativeFixture(t, p))
			s, present := LoadNativeSession("cursor", now)
			if !present || s.Known || !s.QuotaRequired {
				t.Fatalf("unsafe source escaped hold: %+v, present=%v", s, present)
			}
		})
	}
	for _, content := range []string{"{", strings.Repeat("x", (1<<20)+1)} {
		p := filepath.Join(t.TempDir(), "source")
		if err := os.WriteFile(p, []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
		t.Setenv("DOWNSHIFT_CURSOR_NATIVE_FILE", p)
		if s, present := LoadNativeSession("cursor", now); !present || s.Known {
			t.Fatal("malformed/oversized source escaped hold")
		}
	}
	t.Setenv("DOWNSHIFT_CURSOR_NATIVE_FILE", filepath.Join(t.TempDir(), "missing"))
	if s, present := LoadNativeSession("cursor", now); !present || s.Known {
		t.Fatal("missing configured source escaped hold")
	}
	if _, present := LoadNativeSession("codex", now); present {
		t.Fatal("Cursor availability leaked to another harness")
	}
}

type nativeResolver struct {
	Resolver
	models   map[string]Model
	explicit map[string]bool
}

func (r nativeResolver) LookupByID(_, id string) (Model, bool) {
	m, ok := r.models[id]
	return m, ok
}
func (r nativeResolver) IsExplicitOnly(_, id string) bool { return r.explicit[id] }

func TestNativeSelectionUsesRealTierAndQuotaNotProviderOrder(t *testing.T) {
	now := time.Now().UTC()
	p := nativeFixture(now)
	t.Setenv("DOWNSHIFT_CURSOR_NATIVE_FILE", writeNativeFixture(t, p))
	s, _ := LoadNativeSession("cursor", now)
	r := nativeResolver{models: map[string]Model{
		"cheap":  {ID: "cheap", Tier: TierSmall, OutputM: 1},
		"middle": {ID: "middle", Tier: TierMid, OutputM: 2},
		"strong": {ID: "strong", Tier: TierFrontier, OutputM: 3},
	}}
	for _, tier := range []Tier{TierMid, TierFrontier} {
		d := Decision{Harness: "cursor", Tier: tier, Verdict: VerdictUnknown}
		if plan := d.PlanForSession(CursorCaps, r, s); plan.RewriteModel {
			t.Fatalf("exhausted higher-tier pool reclassified cheap as %s: %+v", tier, plan)
		}
	}
	d := Decision{Harness: "cursor", Tier: TierSmall, Verdict: VerdictUnknown}
	if plan := d.PlanForSession(CursorCaps, r, s); !plan.RewriteModel || plan.Model.ID != "cheap" {
		t.Fatalf("funded known-tier model not selected: %+v", plan)
	}
	if CanWriteSessionID("cursor", "unknown", s, r) || CanWriteSessionID("cursor", "cheap", s, nil) {
		t.Fatal("native availability invented capability without metadata")
	}
	p["plan_usage"] = map[string]any{"auto_percent_used": 60, "api_percent_used": 20}
	t.Setenv("DOWNSHIFT_CURSOR_NATIVE_FILE", writeNativeFixture(t, p))
	s, _ = LoadNativeSession("cursor", now)
	d.Tier, d.Verdict = TierMid, VerdictDownshift
	d.CurrentModel = r.models["strong"]
	if plan := d.PlanForSession(CursorCaps, r, s); !plan.RewriteModel || plan.Model.ID != "middle" {
		t.Fatalf("mid floor used provider position rather than metadata: %+v", plan)
	}
	// The current model may be absent from the native candidate set, but its
	// metadata still bounds a downshift after cheaper pools are exhausted.
	d.CurrentModel = r.models["middle"]
	d.Tier = TierSmall
	s.IDs = []string{"strong"}
	if plan := d.PlanForSession(CursorCaps, r, s); plan.RewriteModel {
		t.Fatalf("quota converted downshift into upgrade: %+v", plan)
	}
	s.IDs = []string{"strong", "unknown", "cheap", "middle"}
	d.Tier = TierMid
	d.Verdict = VerdictUpshift
	if plan := d.PlanForSession(CursorCaps, r, s); !plan.RewriteModel || plan.Model.ID != "strong" {
		t.Fatalf("upshift did not choose highest actual tier: %+v", plan)
	}
	r.explicit = map[string]bool{"strong": true}
	t.Setenv("DOWNSHIFT_EXPLICIT_UPSHIFT", "")
	t.Setenv("DOWNSHIFT_SESSION_MODELS", filepath.Join(t.TempDir(), "absent"))
	if plan := d.PlanForSession(CursorCaps, r, s); plan.Model.ID == "strong" {
		t.Fatal("native selection enabled explicit-only upshift by default")
	}
}
