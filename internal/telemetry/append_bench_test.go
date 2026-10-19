// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package telemetry

import (
	"path/filepath"
	"testing"
)

// BenchmarkAppendTo guards the per-spawn cost of one event. With an fsync it
// measured ~2.5 ms/op on macOS (F_FULLFSYNC); without it, ~65 us/op.
func BenchmarkAppendTo(b *testing.B) {
	p := filepath.Join(b.TempDir(), "events.jsonl")
	for i := 0; i < b.N; i++ {
		if err := AppendTo(p, Event{}); err != nil {
			b.Fatal(err)
		}
	}
}
