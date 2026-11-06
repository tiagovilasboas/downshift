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

const (
	maxReportedModels  = 64
	maxReportedIDBytes = 128
)

// splitModels takes the optional "models" array out of raw and returns the
// rest of the arguments plus the cleaned ids (trimmed, empties dropped). An
// absent, null or empty array gives no ids. Messages never echo the values.
func splitModels(raw json.RawMessage, allowed []string) (json.RawMessage, []string, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || string(trimmed) == "null" {
		return raw, nil, nil
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(trimmed, &fields); err != nil {
		return raw, nil, nil // parseArgs reports the malformed object
	}
	v, ok := fields["models"]
	if !ok {
		return raw, nil, nil
	}
	delete(fields, "models")
	rest, err := json.Marshal(fields)
	if err != nil {
		return nil, nil, errors.New("arguments could not be read")
	}
	if string(bytes.TrimSpace(v)) == "null" {
		return rest, nil, nil
	}
	var items []json.RawMessage
	if err := json.Unmarshal(v, &items); err != nil {
		return nil, nil, fmt.Errorf(`field "models" must be an array of strings; allowed fields: %s`, fieldList(allowed))
	}
	if len(items) > maxReportedModels {
		return nil, nil, fmt.Errorf(`field "models" has more than %d items`, maxReportedModels)
	}
	var ids []string
	for _, it := range items {
		var s string
		if err := json.Unmarshal(it, &s); err != nil {
			return nil, nil, errors.New(`field "models" must be an array of strings`)
		}
		if len(s) > maxReportedIDBytes {
			return nil, nil, fmt.Errorf(`field "models" has an item longer than %d bytes`, maxReportedIDBytes)
		}
		if s = strings.TrimSpace(s); s != "" {
			ids = append(ids, s)
		}
	}
	return rest, ids, nil
}
