// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package session

import (
	"reflect"
	"testing"
)

func TestRestrictToAvailable(t *testing.T) {
	cases := []struct {
		name      string
		ids       []string
		available []string
		want      []string
	}{
		{"exact qualified id", []string{"anthropic/claude-x"}, []string{"anthropic/claude-x"}, []string{"anthropic/claude-x"}},
		{"bare id matches its provider entry", []string{"claude-x"}, []string{"anthropic/claude-x"}, []string{"claude-x"}},
		{"id with a slash under a gateway provider", []string{"deepseek/deepseek-v4.1-flash"}, []string{"openrouter/deepseek/deepseek-v4.1-flash"}, []string{"deepseek/deepseek-v4.1-flash"}},
		{"bare id matches any provider", []string{"claude-x"}, []string{"other/claude-x"}, []string{"claude-x"}},
		{"suffix of a slash id is not a match", []string{"deepseek-v4.1-flash"}, []string{"openrouter/deepseek/deepseek-v4.1-flash"}, nil},
		{"unavailable id is dropped", []string{"a", "b"}, []string{"p/b"}, []string{"b"}},
		{"empty available empties the session", []string{"a"}, []string{}, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := KnownSession(tc.ids).RestrictToAvailable(tc.available).IDs
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("IDs = %v, want %v", got, tc.want)
			}
		})
	}
	if got := UnknownSession().RestrictToAvailable([]string{"p/a"}); got.Known {
		t.Fatal("an unknown session must stay unknown")
	}
}
