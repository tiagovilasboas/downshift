// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

// Package gatewaytest holds the contract every gateway adapter must pass.
package gatewaytest

import (
	"testing"

	"github.com/tiagovilasboas/downshift/internal/gateway"
)

// Check runs the port contract against g. samples are canonical "vendor/model"
// ids the gateway is expected to serve. Adapter tests call it with their own
// samples.
func Check(t *testing.T, g gateway.Gateway, samples []string) {
	t.Helper()
	if g.Name() == "" {
		t.Fatal("Name must not be empty")
	}
	for _, id := range samples {
		canonical, ok := g.NormalizeID(id)
		if !ok {
			t.Errorf("NormalizeID(%q) must accept a canonical id", id)
			continue
		}
		if canonical != id {
			t.Errorf("NormalizeID(%q) round trip = %q", id, canonical)
		}
		again, ok := g.NormalizeID(canonical)
		if !ok || again != canonical {
			t.Errorf("NormalizeID is not idempotent for %q: got %q, %v", canonical, again, ok)
		}
	}
	for _, raw := range []string{"", "no-slash-model-id", "a/b/c/d"} {
		if _, ok := g.NormalizeID(raw); ok {
			t.Errorf("NormalizeID(%q) must reject an id that is not a model id", raw)
		}
	}
	const unknown = "gatewaytest-vendor/no-such-model"
	if _, _, known := g.Price(unknown); known {
		t.Errorf("Price(%q) must be unknown for an id the snapshot does not hold", unknown)
	}
	info := g.Privacy(unknown)
	switch info.Retention {
	case gateway.RetentionZero, gateway.RetentionUnknown, gateway.RetentionRetained:
	default:
		t.Errorf("Privacy retention %q is not a known value", info.Retention)
	}
	if info.Source == "" {
		t.Error("Privacy source must name the evidence")
	}
}
