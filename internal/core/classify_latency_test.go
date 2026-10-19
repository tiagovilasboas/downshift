// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package core

import (
	"strings"
	"testing"
	"time"
)

// TestClassifyLatencyLargePrompts guards the hook hot path against input that
// makes classification slow. The hook reads at most 1 MiB, so 900 KB is the
// realistic worst case. The bound is deliberately loose (wall-clock tests are
// flaky on shared runners); it catches order-of-magnitude regressions such as
// backtracking, not small drifts. Measured times are logged for the record.
func TestClassifyLatencyLargePrompts(t *testing.T) {
	skipUnderRace(t)
	filler := "refactor the payment module and update the tests for the token handler. "
	cases := []struct {
		name  string
		bytes int
		limit time.Duration
	}{
		// 1KB and 100KB limits are 5x the laptop figures: 100KB measured 1.66 s
		// under load against a 1 s bound. Regressions of interest are orders of
		// magnitude, so the bound still fails on backtracking.
		{"1KB", 1 << 10, 1 * time.Second},
		{"100KB", 100 << 10, 5 * time.Second},
		// 10 s: the shared CI runner is about 3x slower than a laptop (5.0-5.2 s
		// measured for this size against 1.7 s locally), so 5 s sat on the edge.
		// Backtracking would be orders of magnitude over this.
		{"900KB", 900 << 10, 10 * time.Second},
	}
	for _, c := range cases {
		prompt := strings.Repeat(filler, c.bytes/len(filler)+1)[:c.bytes]
		start := time.Now()
		_ = ClassifyWithSemantic(prompt)
		elapsed := time.Since(start)
		t.Logf("%s: %v", c.name, elapsed)
		if elapsed > c.limit {
			t.Errorf("%s classification took %v, limit %v", c.name, elapsed, c.limit)
		}
	}
}

// TestClassifyLatencyAdversarialInput covers repeated delimiters and long
// unbroken tokens, the usual triggers for regex backtracking.
func TestClassifyLatencyAdversarialInput(t *testing.T) {
	skipUnderRace(t)
	inputs := map[string]string{
		"unclosed_tags": strings.Repeat("<system-reminder>", 20000),
		"one_long_word": strings.Repeat("a", 500<<10),
		"punctuation":   strings.Repeat("(){}[];,.", 50000),
	}
	for name, in := range inputs {
		start := time.Now()
		_ = ClassifyWithSemantic(in)
		elapsed := time.Since(start)
		t.Logf("%s: %v", name, elapsed)
		if elapsed > 5*time.Second {
			t.Errorf("%s took %v", name, elapsed)
		}
	}
}

// skipUnderRace skips wall-clock bounds when the race detector is on; the
// plain `go test` run in ci still enforces them.
func skipUnderRace(t *testing.T) {
	t.Helper()
	if raceEnabled {
		t.Skip("latency bounds do not apply under -race")
	}
}
