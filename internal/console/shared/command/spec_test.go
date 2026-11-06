// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package command

import (
	"reflect"
	"testing"
)

func TestFilterAndExtra(t *testing.T) {
	specs := []Spec{{Args: []string{"report"}, Summary: "tokens"}, {Args: []string{"stats", "--detail"}, Summary: "units"}}
	cases := []struct {
		query string
		want  int
	}{{"", 2}, {"REP", 1}, {"stats units", 1}, {"nope", 0}}
	for _, c := range cases {
		if got := len(Filter(specs, c.query)); got != c.want {
			t.Errorf("Filter(%q) = %d, want %d", c.query, got, c.want)
		}
	}
	if got := WithExtra(specs[0], "report --days=3"); !reflect.DeepEqual(got, []string{"report", "--days=3"}) {
		t.Errorf("WithExtra extra = %v", got)
	}
	if got := WithExtra(specs[0], "rep"); !reflect.DeepEqual(got, []string{"report"}) {
		t.Errorf("WithExtra prefix = %v", got)
	}
	if got := WithExtra(specs[1], "stats --export x"); !reflect.DeepEqual(got, []string{"stats", "--detail"}) {
		t.Errorf("WithExtra mismatch = %v", got)
	}
}
