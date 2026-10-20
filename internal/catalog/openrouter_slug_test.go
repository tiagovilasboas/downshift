// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package catalog

import (
	"encoding/json"
	"testing"
)

// Ids outside the closed character set are rejected, so the whole file is
// refused and no row (live or shadow) is applied. Each case is one row.
func TestParseOpenRouterAllowlist_RejectsIdOutsideCharset(t *testing.T) {
	bad := []string{
		"a/b\nc",           // newline
		"a/b\"x",           // double quote
		"a/b;rm -rf $HOME", // shell metacharacters and whitespace
		"a/`id`",           // backtick
		"a/\x00",           // NUL control character
		"a b/c",            // whitespace in the vendor part
		"~a/b",             // tilde alias in the vendor part
		"a/b c",            // whitespace in the model part
		"a/b/c",            // second slash
		"/b",               // empty vendor
		"a/",               // empty model
		"a/b$(id)",         // command substitution
	}
	for _, id := range bad {
		body, err := json.Marshal(map[string]any{
			"version": 1,
			"harness": "opencode",
			"models": []map[string]string{
				{"id": id, "tier": "small", "max_effort": "low", "route": "live"},
			},
		})
		if err != nil {
			t.Fatal(err)
		}
		if rows, err := parseOpenRouterAllowlist(body); err == nil {
			t.Errorf("id %q: accepted as a live row %+v, want rejection", id, rows)
		}
	}
}

// The real ids the allowlist carries, including the ":free" variant, pass.
func TestParseOpenRouterAllowlist_AcceptsRealIds(t *testing.T) {
	for _, id := range []string{
		"deepseek/deepseek-v4.1-flash-1017",
		"minimax/minimax-m3.1-flash:free",
		"x-ai/grok_x",
	} {
		body := []byte(`{"version":1,"harness":"opencode","models":[{"id":"` + id + `","tier":"small","max_effort":"low"}]}`)
		if _, err := parseOpenRouterAllowlist(body); err != nil {
			t.Errorf("id %q rejected: %v", id, err)
		}
	}
}
