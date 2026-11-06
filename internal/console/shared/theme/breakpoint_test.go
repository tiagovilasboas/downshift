// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package theme

import "testing"

func TestBreakpointFor(t *testing.T) {
	cases := []struct {
		width int
		want  Breakpoint
	}{
		{-1, Normal}, {0, Normal}, {1, Narrow}, {59, Narrow},
		{60, Normal}, {80, Normal}, {99, Normal}, {100, Wide}, {200, Wide},
	}
	for _, c := range cases {
		if got := BreakpointFor(c.width); got != c.want {
			t.Errorf("BreakpointFor(%d) = %d, want %d", c.width, got, c.want)
		}
	}
}

func TestInfoRoleFollowsNoColor(t *testing.T) {
	if got := build(true).Info.Render("x"); got != "x" {
		t.Errorf("Info under NO_COLOR = %q, want plain", got)
	}
	if got := build(false).Info.Render("x"); got == "x" {
		t.Error("Info with colour should style its text")
	}
}
