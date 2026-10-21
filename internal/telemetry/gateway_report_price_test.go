// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package telemetry

import (
	"bytes"
	"strings"
	"testing"
)

// TestReport_GatewayRunWithoutSnapshotPrintsUnknown: a gateway run with no
// snapshot price is printed with price source unknown and is never priced at a
// tier price; a run the snapshot prices is printed as snapshot.
func TestReport_GatewayRunWithoutSnapshotPrintsUnknown(t *testing.T) {
	events := []Event{
		{Harness: "claude-code", Outcome: OutcomeUsage, LinkedDecision: "g1", ToModel: "openai/gpt-x", GatewayPrice: GatewayPriceUnknown},
		{Harness: "claude-code", Outcome: OutcomeUsage, LinkedDecision: "g2", ToModel: "openai/gpt-y", GatewayPrice: GatewayPriceSnapshot},
		{Harness: "claude-code", Outcome: OutcomeUsage, LinkedDecision: "n1", ToModel: "claude-haiku-4-5"},
	}
	rep := BuildReport(events, 0)
	if rep.GatewayPriceSources[GatewayPriceUnknown] != 1 || rep.GatewayPriceSources[GatewayPriceSnapshot] != 1 {
		t.Fatalf("sources = %v, want one unknown and one snapshot", rep.GatewayPriceSources)
	}
	var out bytes.Buffer
	PrintReport(&out, rep)
	line := "Gateway runs: 2 (price source: snapshot 1, unknown 1); never priced at a tier price"
	if !strings.Contains(out.String(), line) {
		t.Fatalf("report lacks the price source line %q:\n%s", line, out.String())
	}
}

// TestReport_NoGatewayRunsNoPriceLine: a window with no gateway runs prints no
// price source line.
func TestReport_NoGatewayRunsNoPriceLine(t *testing.T) {
	events := []Event{{Harness: "claude-code", Outcome: OutcomeUsage, LinkedDecision: "n1", ToModel: "claude-haiku-4-5"}}
	rep := BuildReport(events, 0)
	if rep.GatewayPriceSources != nil {
		t.Fatalf("sources = %v, want none", rep.GatewayPriceSources)
	}
	var out bytes.Buffer
	PrintReport(&out, rep)
	if strings.Contains(out.String(), "price source") {
		t.Fatalf("unexpected price line:\n%s", out.String())
	}
}
