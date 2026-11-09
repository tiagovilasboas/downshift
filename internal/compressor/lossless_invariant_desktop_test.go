// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor_test

import (
	"bytes"
	"encoding/json"
	"strconv"
	"strings"
)

// agentDesktopInvariant: every element the agent can act on keeps its ref
// (eN under the legend, or the whole ref) and every name keeps its text. The
// full field-by-field proof is the decoder in agent_desktop_roundtrip_test.go.
//
// Text that is not one JSON value (a warning line before the envelope, say)
// is not a shape the view reads: it must come back byte for byte.
func agentDesktopInvariant(in, out []byte) []string {
	if !json.Valid(bytes.TrimSpace(in)) {
		if !bytes.Equal(in, out) {
			return []string{"input is not one JSON envelope, but the output changed it"}
		}
		return nil
	}
	var env struct {
		Data struct {
			SnapshotID string `json:"snapshot_id"`
		} `json:"data"`
	}
	_ = json.Unmarshal(in, &env)
	var lost []string
	var walk func(v any)
	walk = func(v any) {
		switch t := v.(type) {
		case map[string]any:
			if ref, ok := t["ref_id"].(string); ok {
				short := strings.TrimPrefix(ref, "@"+env.Data.SnapshotID+":")
				if !bytes.Contains(out, []byte(short+" ")) && !bytes.Contains(out, []byte(strconv.Quote(ref))) {
					lost = append(lost, "ref "+ref)
				}
			}
			if name, ok := t["name"].(string); ok && !bytes.Contains(out, []byte(strconv.Quote(name))) {
				lost = append(lost, "name "+name)
			}
			for _, c := range t {
				walk(c)
			}
		case []any:
			for _, c := range t {
				walk(c)
			}
		}
	}
	var whole any
	_ = json.Unmarshal(in, &whole)
	walk(whole)
	return lost
}
