// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package childend

import (
	"testing"
	"time"

	"github.com/tiagovilasboas/downshift/internal/catalog"
	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

// downshift report keeps only priced completed tasks. With no usage model and
// no known requested model, a control spawn is unpriced (dropped from the
// control arm) while a routed rewrite of the same spawn is priced on its
// rewrite target (kept in the routed arm): the arms are filtered differently.
func TestHonesty_ReportArmsPricedAsymmetrically(t *testing.T) {
	r := catalog.Load()
	u := telemetry.TokenUsage{InputTokens: 1000, OutputTokens: 500}
	control := &telemetry.Event{CorrelationID: "c1", Harness: "claude-code", Outcome: telemetry.OutcomeBaseline,
		FromModel: "unknown", ToModel: "unknown", ReasonCodes: []string{telemetry.ReasonControl}}
	routed := &telemetry.Event{CorrelationID: "r1", Harness: "claude-code", Outcome: telemetry.OutcomeRewriteEmitted,
		Verdict: "DOWNSHIFT", FromModel: "unknown", ToModel: "claude-haiku-5-5"}
	c := buildUsageEvent("claude-code", u, "", "s", "t", control, r, time.Now())
	ro := buildUsageEvent("claude-code", u, "", "s", "t", routed, r, time.Now())
	if (c.ActualCostUSD == nil) != (ro.ActualCostUSD == nil) {
		t.Fatalf("same unknown-model spawn: control priced=%v, routed priced=%v; report drops it from one arm only",
			c.ActualCostUSD != nil, ro.ActualCostUSD != nil)
	}
}
