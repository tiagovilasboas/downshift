// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package tools

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// parseArgs reads a flat object of optional string fields. Empty or null input
// is {}. A field outside allowed, a non-object or a non-string value is
// refused with a message that names the allowed fields and nothing the caller
// sent.
func parseArgs(raw json.RawMessage, allowed []string) (map[string]string, error) {
	out := map[string]string{}
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || string(raw) == "null" {
		return out, nil
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return nil, fmt.Errorf("arguments must be a JSON object; allowed fields: %s", fieldList(allowed))
	}
	for k, v := range fields {
		if !contains(allowed, k) {
			return nil, unknownFieldErr(k, allowed)
		}
		if string(bytes.TrimSpace(v)) == "null" {
			continue
		}
		var s string
		if err := json.Unmarshal(v, &s); err != nil {
			return nil, fmt.Errorf("field %q must be a string", k)
		}
		if limit := maxArgBytes(k); len(s) > limit {
			return nil, fmt.Errorf("field %q is longer than %d bytes", k, limit)
		}
		out[k] = s
	}
	return out, nil
}

// maxArgBytes is the longest value a string argument may have.
func maxArgBytes(field string) int {
	if field == "dir" {
		return 4096
	}
	return 256
}

func unknownFieldErr(field string, allowed []string) error {
	if field == "prompt" {
		return fmt.Errorf("this tool never takes task text; allowed fields: %s", fieldList(allowed))
	}
	return fmt.Errorf("unknown field; allowed fields: %s", fieldList(allowed))
}

func fieldList(allowed []string) string {
	if len(allowed) == 0 {
		return "none"
	}
	return strings.Join(allowed, ", ")
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

// harnessIDs returns the valid harness ids, or nil when none were declared.
func (d Deps) harnessIDs() []string {
	if d.Harnesses == nil {
		return nil
	}
	return d.Harnesses()
}

// checkHarness requires a known harness id.
func (d Deps) checkHarness(id string) error {
	if id == "" {
		return errors.New(`field "harness" is required`)
	}
	if d.Harnesses == nil || contains(d.Harnesses(), id) {
		return nil
	}
	return fmt.Errorf("unknown harness; valid harnesses: %s", fieldList(d.Harnesses()))
}
