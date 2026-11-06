// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package docscheck_test

import (
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/gateway/ninerouter"
)

var localEndpoint = regexp.MustCompile(`(?:localhost|127\.0\.0\.1):\d+`)

// exampleEndpoints are ports docs may show that are not a Downshift default:
// the port of a tool the docs only mention in passing.
var exampleEndpoints = map[string]bool{}

// endpointProblems returns the local endpoints in lines that are neither a
// default found in the code nor allowlisted. A line that names 9router must
// use the gateway default.
func endpointProblems(lines []docLine, defaults map[string]bool, gateway string) []string {
	var bad []string
	for _, l := range lines {
		for _, ep := range localEndpoint.FindAllString(l.Text, -1) {
			host := "localhost:" + ep[strings.LastIndex(ep, ":")+1:]
			where := l.File + ":" + strconv.Itoa(l.N) + " " + ep
			if strings.Contains(strings.ToLower(l.Text), "9router") && host != gateway {
				bad = append(bad, where+" (9router default is "+gateway+")")
			} else if !defaults[host] && !exampleEndpoints[host] {
				bad = append(bad, where+" (not a default in the code)")
			}
		}
	}
	return bad
}

// codeEndpoints are the local endpoints written in non-test Go sources.
func codeEndpoints(t *testing.T, root string) map[string]bool {
	t.Helper()
	out := map[string]bool{}
	for _, f := range sourceFiles(t, root, []string{".go"}, false) {
		for _, ep := range localEndpoint.FindAllString(mustRead(t, f), -1) {
			out["localhost:"+ep[strings.LastIndex(ep, ":")+1:]] = true
		}
	}
	return out
}

// Every localhost endpoint a doc gives is a default the code has, and the
// 9router one is the gateway constant.
func TestDocumentedEndpointsMatchTheCode(t *testing.T) {
	root := repoRoot(t)
	gateway := localEndpoint.FindString(ninerouter.DefaultEndpoint)
	defaults := codeEndpoints(t, root)
	if !defaults[gateway] {
		t.Fatalf("gateway default %s not found in the Go sources", gateway)
	}
	for _, b := range endpointProblems(guardedLines(t, root), defaults, gateway) {
		t.Errorf("%s; fix the doc or mark the line %s", b, historyMarker)
	}
}

func TestEndpointGuardBites(t *testing.T) {
	defaults := map[string]bool{"localhost:20128": true}
	old := []docLine{{"x.md", 1, "9router at http://localhost:3000"}, {"x.md", 2, "other 127.0.0.1:9999"}}
	if got := endpointProblems(old, defaults, "localhost:20128"); len(got) != 2 {
		t.Errorf("got %v, want both flagged", got)
	}
	ok := []docLine{{"x.md", 1, "9router at http://localhost:20128"}}
	if got := endpointProblems(ok, defaults, "localhost:20128"); len(got) != 0 {
		t.Errorf("got %v, want none", got)
	}
}
