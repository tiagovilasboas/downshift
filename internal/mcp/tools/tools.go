// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

// Package tools is the port between the MCP delegation server and what it
// offers: a tool is a name, a description, an input schema and a plain Go
// function. Nothing here knows the protocol or the library that speaks it; the
// transport package adapts these tools to the wire
// (docs/internal/plans/mcp-delegation-spec.md).
//
// Downshift by Tiago Vilas Boas
// https://github.com/tiagovilasboas/downshift
package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
)

// Handler runs one tool call. args is the JSON the caller sent, possibly empty;
// the handler validates it. A returned error is reported to the caller as a
// failed tool result, not as a protocol error, so the model can read the
// message and correct itself. The message must not contain task text.
type Handler func(ctx context.Context, args json.RawMessage) (Result, error)

// Result is the text a tool returns. IsError marks a failure the handler chose
// to report as a result.
type Result struct {
	Text    string
	IsError bool
}

// Tool is one callable offered to the orchestrator.
type Tool struct {
	Name        string
	Description string
	// InputSchema is a JSON Schema object. Empty means any object.
	InputSchema json.RawMessage
	Handler     Handler
}

// AnyObject is the schema of a tool that takes no arguments, or any.
var AnyObject = json.RawMessage(`{"type":"object"}`)

// Schema returns the tool's input schema, defaulting to any object.
func (t Tool) Schema() json.RawMessage {
	if len(t.InputSchema) == 0 {
		return AnyObject
	}
	return t.InputSchema
}

// Validate refuses a tool set that could not be served: an empty or duplicate
// name, a missing handler, or an input schema that is not a JSON object with
// "type": "object".
func Validate(ts []Tool) error {
	seen := map[string]bool{}
	for _, t := range ts {
		switch {
		case t.Name == "":
			return errors.New("a tool has no name")
		case seen[t.Name]:
			return fmt.Errorf("duplicate tool name %q", t.Name)
		case t.Handler == nil:
			return fmt.Errorf("tool %q has no handler", t.Name)
		}
		seen[t.Name] = true
		var schema struct {
			Type string `json:"type"`
		}
		if err := json.Unmarshal(t.Schema(), &schema); err != nil || schema.Type != "object" {
			return fmt.Errorf("tool %q: input schema must be a JSON object with type \"object\"", t.Name)
		}
	}
	return nil
}
