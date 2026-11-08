// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package accounting

import (
	"testing"
	"time"

	"github.com/tiagovilasboas/downshift-ai/internal/execwrap"
)

func TestDirectCompactorUsesGivenNow(t *testing.T) {
	ev := execwrap.Event{TS: "2026-11-01T12:00:00Z", Compacted: true, RawBytes: 4000, EmittedBytes: 400}
	scope := Scope{Kind: ScopeDays, Days: 7}
	cases := []struct {
		name string
		now  string
		want int64
	}{
		{"inside window", "2026-11-05T00:00:00Z", 1},
		{"after window", "2026-11-20T00:00:00Z", 0},
	}
	for _, c := range cases {
		now, _ := time.Parse(time.RFC3339, c.now)
		if got := DirectCompactor([]execwrap.Event{ev}, scope, now).Commands; got != c.want {
			t.Errorf("%s: commands = %d, want %d", c.name, got, c.want)
		}
	}
}
