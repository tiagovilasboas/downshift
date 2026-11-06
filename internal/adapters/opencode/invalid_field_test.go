// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package opencode

import (
	"errors"
	"strings"
	"testing"
)

func TestHandle_NamesTheInvalidField(t *testing.T) {
	base := func() Request {
		return Request{V: 1, Kind: kindRoute, Harness: RequestHarness{ID: harnessID}, Event: eventSpawn,
			Correlation: RequestRelation{Child: strings.Repeat("a", 16)},
			Model:       RequestModel{ID: "m"}, TaskText: "task"}
	}
	cases := []struct {
		name   string
		mutate func(*Request)
		want   string
	}{
		{"wrong version", func(r *Request) { r.V = 2 }, "v"},
		{"empty model id", func(r *Request) { r.Model.ID = "" }, "model.id"},
		{"bad child id", func(r *Request) { r.Correlation.Child = "XYZ" }, "correlation.child"},
		{"empty task", func(r *Request) { r.TaskText = "" }, "taskText"},
		{"long agent", func(r *Request) { r.Agent = strings.Repeat("x", maxAgentLen+1) }, "agent"},
		{"empty available entry", func(r *Request) { r.Available = []string{""} }, "available"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := base()
			tc.mutate(&req)
			resp, _, _ := Handle(req)
			if resp.ReasonCode != ReasonFailOpen || resp.InvalidField != tc.want {
				t.Fatalf("reason %q field %q, want fail_open %q", resp.ReasonCode, resp.InvalidField, tc.want)
			}
		})
	}
}

func TestParseRequest_NamesTheFieldNotTheValue(t *testing.T) {
	_, err := ParseRequest([]byte(`{"v":"SECRET-VALUE","model":{"id":1}}`))
	var named interface{ InvalidField() string }
	if !errors.As(err, &named) || named.InvalidField() != "v" {
		t.Fatalf("err %v does not name field v", err)
	}
}
