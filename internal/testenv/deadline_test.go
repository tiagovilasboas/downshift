// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package testenv

import (
	"testing"
	"time"
)

func TestScaled(t *testing.T) {
	cases := []struct {
		env  string
		want time.Duration
	}{
		{"", 2 * time.Second},
		{"1", 2 * time.Second},
		{"4", 8 * time.Second},
		{"1.5", 3 * time.Second},
		{"0.5", 2 * time.Second}, // never shrinks a deadline
		{"-3", 2 * time.Second},
		{"fast", 2 * time.Second},
	}
	for _, tc := range cases {
		t.Setenv(TimeoutScaleEnv, tc.env)
		if got := Scaled(2 * time.Second); got != tc.want {
			t.Errorf("%s=%q: Scaled(2s) = %v, want %v", TimeoutScaleEnv, tc.env, got, tc.want)
		}
	}
}
