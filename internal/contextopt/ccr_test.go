// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package contextopt

import (
	"errors"
	"testing"
)

func TestCCREnabled(t *testing.T) {
	yes, no := true, false
	tests := []struct {
		name string
		cfg  Config
		err  error
		env  string
		want bool
	}{
		{"absent is on", Config{}, nil, "", true},
		{"file true", Config{CCR: &yes}, nil, "", true},
		{"file false", Config{CCR: &no}, nil, "", false},
		{"env on wins", Config{CCR: &no}, nil, "1", true},
		{"env off wins", Config{CCR: &yes}, nil, "0", false},
		{"cfg error", Config{CCR: &yes}, errors.New("x"), "", false},
		{"env on beats cfg error", Config{}, errors.New("x"), "1", true},
		{"other env ignored", Config{CCR: &yes}, nil, "maybe", true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := CCREnabled(tc.cfg, tc.err, tc.env); got != tc.want {
				t.Fatalf("got %v want %v", got, tc.want)
			}
		})
	}
}
