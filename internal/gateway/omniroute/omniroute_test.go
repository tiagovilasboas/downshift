// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package omniroute

import (
	"context"
	"errors"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/gateway"
	"github.com/tiagovilasboas/downshift/internal/gateway/gatewaytest"
)

func fixedSnapshot(model string) (float64, float64, bool) {
	if model == "vendor/priced" {
		return 0.5, 1.5, true
	}
	return 0, 0, false
}

func TestContract(t *testing.T) {
	gatewaytest.Check(t, New(fixedSnapshot), []string{"cc/claude-opus-4-6", "openai/gpt-5.4", "gh/gpt-5.5"})
	gatewaytest.Check(t, New(nil), []string{"vendor/model"})
}

func TestNameIsRegistryKey(t *testing.T) {
	if got := New(nil).Name(); got != "omniroute" {
		t.Fatalf("Name() = %q, want omniroute", got)
	}
}

func TestNormalizeRejectsRoutingAliasesAndMultiSlash(t *testing.T) {
	for _, raw := range []string{"auto", "auto/coding", "auto/fast", "nebius/Qwen/Qwen3-Embedding-8B"} {
		if _, ok := New(nil).NormalizeID(raw); ok {
			t.Errorf("NormalizeID(%q) must reject a routing alias or a multi-slash id", raw)
		}
	}
}

func TestPriceWithoutSnapshotIsUnknown(t *testing.T) {
	if _, _, ok := New(nil).Price("vendor/priced"); ok {
		t.Fatal("a gateway with no snapshot must price nothing")
	}
	if in, out, ok := New(fixedSnapshot).Price("vendor/priced"); !ok || in != 0.5 || out != 1.5 {
		t.Fatalf("Price from snapshot = %v/%v/%v", in, out, ok)
	}
}

func TestPrivacyDefaultIsUnknown(t *testing.T) {
	got := New(nil).Privacy("cc/claude-opus-4-6")
	if got.Retention != gateway.RetentionUnknown {
		t.Fatalf("Retention = %q, want unknown by default", got.Retention)
	}
	if got.Source == "" {
		t.Fatal("Privacy source must name the evidence")
	}
}

func TestListModelsIsUnsupported(t *testing.T) {
	_, err := New(nil).ListModels(context.Background())
	if !errors.Is(err, gateway.ErrUnsupported) {
		t.Fatalf("ListModels error = %v, want ErrUnsupported", err)
	}
}
