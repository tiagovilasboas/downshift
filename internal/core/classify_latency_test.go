// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
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
	filler := "refactor the payment module and update the tests for the token handler. "
	cases := []struct {
		name  string
		bytes int
		limit time.Duration
	}{
		{"1KB", 1 << 10, 200 * time.Millisecond},
		{"100KB", 100 << 10, 1 * time.Second},
		{"900KB", 900 << 10, 5 * time.Second},
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
