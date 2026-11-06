// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package catalog

import "testing"

// The route field decides the routing of an allowlisted row: absent or
// "shadow" is observation only, "live" is a rewrite target, anything else
// rejects the whole file.
func TestParseOpenRouterAllowlist_RouteField(t *testing.T) {
	cases := []struct {
		route   string
		want    string
		wantErr bool
	}{
		{route: "", want: RoutingShadow},
		{route: "shadow", want: RoutingShadow},
		{route: "live", want: RoutingLive},
		{route: "Live", wantErr: true},
		{route: "always", wantErr: true},
	}
	for _, tc := range cases {
		body := `{"version":1,"harness":"opencode","models":[{"id":"a/b","tier":"small","max_effort":"low","route":"` + tc.route + `"}]}`
		rows, err := parseOpenRouterAllowlist([]byte(body))
		if tc.wantErr {
			if err == nil {
				t.Errorf("route %q: want an error, got rows %+v", tc.route, rows)
			}
			continue
		}
		if err != nil {
			t.Errorf("route %q: unexpected error %v", tc.route, err)
			continue
		}
		if len(rows) != 1 || rows[0].Routing != tc.want {
			t.Errorf("route %q: routing = %+v, want %s", tc.route, rows, tc.want)
		}
	}
}
