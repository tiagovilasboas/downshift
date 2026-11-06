// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package ccr

import "testing"

const sample = "alpha\nbeta\ngamma\nbeta2\nomega\n"

func TestSlice(t *testing.T) {
	tests := []struct {
		name string
		q    Query
		want string
	}{
		{"none", Query{}, sample},
		{"head", Query{Head: 2}, "alpha\nbeta\n"},
		{"tail", Query{Tail: 2}, "beta2\nomega\n"},
		{"head over", Query{Head: 99}, sample},
		{"grep", Query{Grep: "^beta"}, "beta\nbeta2\n"},
		{"lines", Query{From: 2, To: 3}, "2:beta\n3:gamma\n"},
		{"from only", Query{From: 4}, "4:beta2\n5:omega\n"},
		{"to only", Query{To: 1}, "1:alpha\n"},
		{"grep head", Query{Grep: "beta", Head: 1}, "beta\n"},
		{"grep tail", Query{Grep: "beta", Tail: 1}, "beta2\n"},
		{"lines grep", Query{From: 2, To: 4, Grep: "^g"}, "3:gamma\n"},
		{"lines head", Query{From: 2, Head: 2}, "2:beta\n3:gamma\n"},
		{"head then tail", Query{Head: 4, Tail: 2}, "gamma\nbeta2\n"},
		{"no match", Query{Grep: "zzz"}, ""},
		{"empty range", Query{From: 9}, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Slice([]byte(sample), tc.q)
			if err != nil || string(got) != tc.want {
				t.Fatalf("got %q err %v, want %q", got, err, tc.want)
			}
		})
	}
}

func TestSliceBadRegexp(t *testing.T) {
	if _, err := Slice([]byte(sample), Query{Grep: "("}); err == nil {
		t.Fatal("want error")
	}
}
