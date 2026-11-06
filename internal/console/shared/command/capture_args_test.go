// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package command

import (
	"reflect"
	"testing"
)

func TestCaptureArgs(t *testing.T) {
	cases := []struct{ in, want []string }{
		{[]string{"report"}, []string{"report", "--text"}},
		{[]string{"report", "--days=7"}, []string{"report", "--days=7", "--text"}},
		{[]string{"report", "--json"}, []string{"report", "--json"}},
		{[]string{"report", "--text"}, []string{"report", "--text"}},
		{[]string{"report", "clear"}, []string{"report", "clear"}},
		{[]string{"session", "--brief"}, []string{"session", "--brief"}},
		{nil, nil},
	}
	for _, c := range cases {
		if got := captureArgs(c.in); !reflect.DeepEqual(got, c.want) {
			t.Errorf("captureArgs(%v) = %v, want %v", c.in, got, c.want)
		}
	}
}
