// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package tools

import (
	"context"
	"encoding/json"
	"testing"
)

func ok(context.Context, json.RawMessage) (Result, error) { return Result{Text: "ok"}, nil }

func TestValidate(t *testing.T) {
	cases := []struct {
		name string
		ts   []Tool
		ok   bool
	}{
		{"one tool with the default schema", []Tool{{Name: "a", Handler: ok}}, true},
		{"explicit object schema", []Tool{{Name: "a", Handler: ok, InputSchema: json.RawMessage(`{"type":"object","properties":{"x":{"type":"string"}}}`)}}, true},
		{"empty name", []Tool{{Handler: ok}}, false},
		{"duplicate name", []Tool{{Name: "a", Handler: ok}, {Name: "a", Handler: ok}}, false},
		{"no handler", []Tool{{Name: "a"}}, false},
		{"schema is not an object", []Tool{{Name: "a", Handler: ok, InputSchema: json.RawMessage(`{"type":"string"}`)}}, false},
		{"schema is not json", []Tool{{Name: "a", Handler: ok, InputSchema: json.RawMessage(`nope`)}}, false},
		{"no tools is valid", nil, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if err := Validate(c.ts); (err == nil) != c.ok {
				t.Fatalf("Validate = %v, want ok=%v", err, c.ok)
			}
		})
	}
}

func TestSchemaDefaultsToAnyObject(t *testing.T) {
	if got := string((Tool{}).Schema()); got != `{"type":"object"}` {
		t.Fatalf("default schema = %s", got)
	}
}
