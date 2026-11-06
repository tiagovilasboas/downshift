// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package classify

import "testing"

func TestDecisive(t *testing.T) {
	cases := []struct {
		name string
		c    Classification
		want bool
	}{
		{"confident with a signal", Classification{Confident: true}, true},
		{"not confident", Classification{}, false},
		{"no content signal", Classification{Confident: true, NoSignal: true}, false},
		{"risk floor lifted it", Classification{Confident: true, RiskFloor: true}, false},
		{"risk rule cleared it", Classification{Confident: true, RiskAbstain: true}, false},
	}
	for _, c := range cases {
		if got := c.c.Decisive(); got != c.want {
			t.Errorf("%s: Decisive = %v, want %v", c.name, got, c.want)
		}
	}
}
