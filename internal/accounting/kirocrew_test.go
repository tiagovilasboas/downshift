// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package accounting

import (
	"testing"
)

func TestEstimateContextSavings(t *testing.T) {
	cases := []struct {
		name           string
		compactorSaved int64
		contextUsed    int64
		wantApprox     float64
	}{
		{
			name:           "typical savings",
			compactorSaved: 40000, // 10k tokens
			contextUsed:    90000,
			// hypothetical = 90000 + 10000 = 100000
			// savings = 10000 / 100000 * 100 = 10%
			wantApprox: 10.0,
		},
		{
			name:           "no context used",
			compactorSaved: 4000,
			contextUsed:    0,
			wantApprox:     0,
		},
		{
			name:           "no savings",
			compactorSaved: 0,
			contextUsed:    100000,
			wantApprox:     0,
		},
		{
			name:           "high savings ratio",
			compactorSaved: 200000, // 50k tokens
			contextUsed:    50000,
			// hypothetical = 50000 + 50000 = 100000
			// savings = 50000 / 100000 * 100 = 50%
			wantApprox: 50.0,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := EstimateContextSavings(tc.compactorSaved, tc.contextUsed)
			// Allow 0.1% tolerance for floating point
			if got < tc.wantApprox-0.1 || got > tc.wantApprox+0.1 {
				t.Errorf("EstimateContextSavings(%d, %d) = %.2f; want ~%.2f",
					tc.compactorSaved, tc.contextUsed, got, tc.wantApprox)
			}
		})
	}
}
