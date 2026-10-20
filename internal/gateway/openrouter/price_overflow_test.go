// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package openrouter

import (
	"math"
	"testing"
)

func TestPerMillionOverflowIsUnknownNotInfinite(t *testing.T) {
	for _, s := range []string{"1e308", "NaN", "-1", "Inf", "1e400"} {
		if got := perMillion(s); got != nil {
			t.Errorf("perMillion(%q) = %v, want nil", s, *got)
		}
	}
}

func TestPerMillionScaledFiniteValueIsKept(t *testing.T) {
	got := perMillion("1e300")
	if got == nil || math.IsInf(*got, 0) || *got != 1e306 {
		t.Fatalf("perMillion(1e300) = %v, want 1e306", got)
	}
}
