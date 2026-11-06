// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package policy

import (
	"encoding/json"
	"os"
	"strings"
)

// orchestrationField returns the trimmed raw JSON of one key under
// "orchestration" in a policy file. An absent, oversized or unreadable file, or
// an absent key, returns "". An orchestration that is not an object returns a
// note naming the key it ignored.
func orchestrationField(path, key string) (string, string) {
	if path == "" {
		return "", ""
	}
	fi, err := os.Lstat(path)
	if err != nil || !fi.Mode().IsRegular() || fi.Size() > maxBytes {
		return "", ""
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", ""
	}
	var doc struct {
		Orchestration json.RawMessage `json:"orchestration"`
	}
	if json.Unmarshal(data, &doc) != nil || doc.Orchestration == nil {
		return "", ""
	}
	var o map[string]json.RawMessage
	if json.Unmarshal(doc.Orchestration, &o) != nil {
		return "", path + ": orchestration must be an object; " + key + " ignored"
	}
	return strings.TrimSpace(string(o[key])), ""
}
