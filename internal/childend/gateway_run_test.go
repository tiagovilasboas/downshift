// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package childend_test

import (
	"path/filepath"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/catalog"
	"github.com/tiagovilasboas/downshift/internal/childend"
	"github.com/tiagovilasboas/downshift/internal/hookport"
	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

// A usage record of a child that ran through an OpenRouter route names the
// slug that ran and its price source; a native child keeps its model and gets
// no price source.
func TestRecord_GatewayRunNamesTheModelThatRan(t *testing.T) {
	prev := telemetry.GatewayPriceLookup
	t.Cleanup(func() { telemetry.GatewayPriceLookup = prev })
	telemetry.GatewayPriceLookup = func(model string) (float64, float64, bool) {
		return 0.1, 0.2, model == "vendor/priced-model"
	}
	cases := []struct {
		name, model, resolved, wantModel, wantPrice string
	}{
		{name: "gateway with a snapshot price", model: "claude-haiku-4-5", resolved: "vendor/priced-model", wantModel: "vendor/priced-model", wantPrice: telemetry.GatewayPriceSnapshot},
		{name: "gateway without a snapshot price", model: "claude-haiku-4-5", resolved: "vendor/other-model", wantModel: "vendor/other-model", wantPrice: telemetry.GatewayPriceUnknown},
		{name: "native run", model: "claude-haiku-4-5", resolved: "claude-haiku-4-5", wantModel: "claude-haiku-4-5"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("DOWNSHIFT_EVENT_LOG", filepath.Join(t.TempDir(), "events.jsonl"))
			res := childend.Record(hookport.ChildEnd{
				Harness:       "claude-code",
				SessionID:     "sess-gateway",
				ToolUseID:     "tool-gateway",
				Model:         tc.model,
				ResolvedModel: tc.resolved,
				Usage:         &hookport.TokenUsage{InputTokens: 100, OutputTokens: 50},
			}, "test", catalog.Load())
			if res.Usage == nil {
				t.Fatal("no usage record written")
			}
			if res.Usage.ToModel != tc.wantModel {
				t.Fatalf("ran model %q, want %q", res.Usage.ToModel, tc.wantModel)
			}
			if res.Usage.GatewayPrice != tc.wantPrice {
				t.Fatalf("gateway price %q, want %q", res.Usage.GatewayPrice, tc.wantPrice)
			}
		})
	}
}
