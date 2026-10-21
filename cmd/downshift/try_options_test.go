// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"reflect"
	"testing"
)

func TestStripTryOptions(t *testing.T) {
	cases := []struct {
		name    string
		in      []string
		rest    []string
		harness string
		model   string
		bad     string
	}{
		{"flags before prompt", []string{"--harness", "opencode", "rename x"}, []string{"rename x"}, "opencode", "", ""},
		{"flags after prompt", []string{"rename x", "--harness=opencode", "--model", "glm"}, []string{"rename x"}, "opencode", "glm", ""},
		{"positional kept", []string{"rename x", "opencode", "glm"}, []string{"rename x", "opencode", "glm"}, "", "", ""},
		{"unknown leading flag", []string{"--bogus", "rename x"}, nil, "", "", "--bogus"},
		{"missing value", []string{"rename x", "--model"}, nil, "", "", "--model"},
	}
	for _, c := range cases {
		rest, h, m, bad := stripTryOptions(c.in)
		if !reflect.DeepEqual(rest, c.rest) || h != c.harness || m != c.model || bad != c.bad {
			t.Errorf("%s: got %v %q %q %q", c.name, rest, h, m, bad)
		}
	}
}
