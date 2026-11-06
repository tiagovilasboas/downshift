// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package contextopt

import (
	"errors"
	"testing"
)

func TestCCRFailuresEnabled(t *testing.T) {
	yes, no := true, false
	tests := []struct {
		name        string
		cfg         Config
		err         error
		env, ccrEnv string
		want        bool
	}{
		{"default on", Config{}, nil, "", "", true},
		{"file false", Config{CCRFailures: &no}, nil, "", "", false},
		{"file true", Config{CCRFailures: &yes}, nil, "", "", true},
		{"env off wins", Config{CCRFailures: &yes}, nil, "0", "", false},
		{"env on wins", Config{CCRFailures: &no}, nil, "1", "", true},
		{"ccr off disables", Config{}, nil, "", "0", false},
		{"config error keeps default", Config{}, errors.New("x"), "", "", true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := CCRFailuresEnabled(tc.cfg, tc.err, tc.env, tc.ccrEnv); got != tc.want {
				t.Fatalf("got %v want %v", got, tc.want)
			}
		})
	}
}
