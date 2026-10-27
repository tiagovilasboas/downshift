// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package telemetry_test

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

// Event logs written before the tokens-only cut carry dollar keys. Nothing
// writes or reads them now, but such a line must still decode, to the same
// event as the same line without them.
func TestLegacyDollarKeysDecodeAsIfAbsent(t *testing.T) {
	const withKeys = `{"correlation_id":"c1","timestamp":"2026-10-13T10:01:00Z","harness":"claude-code","outcome":"usage","linked_decision":"d1","input_tokens":1000,"output_tokens":200,"actual_cost_usd":0.25,"baseline_cost_usd":0.5}`
	const withoutKeys = `{"correlation_id":"c1","timestamp":"2026-10-13T10:01:00Z","harness":"claude-code","outcome":"usage","linked_decision":"d1","input_tokens":1000,"output_tokens":200}`

	dir := t.TempDir()
	legacy := filepath.Join(dir, "legacy.jsonl")
	clean := filepath.Join(dir, "clean.jsonl")
	if err := os.WriteFile(legacy, []byte(withKeys+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(clean, []byte(withoutKeys+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := telemetry.ReadEventsFrom(legacy)
	if err != nil {
		t.Fatalf("legacy line did not decode: %v", err)
	}
	want, err := telemetry.ReadEventsFrom(clean)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || len(want) != 1 {
		t.Fatalf("got %d legacy and %d clean events, want 1 each", len(got), len(want))
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("legacy line decodes differently from the same line without dollar keys:\n got %+v\nwant %+v", got[0], want[0])
	}
	if got[0].InputTokens == nil || *got[0].InputTokens != 1000 || got[0].Outcome != telemetry.OutcomeUsage {
		t.Fatalf("token usage lost from a legacy line: %+v", got[0])
	}
}
