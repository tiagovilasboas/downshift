// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package catalog

import "testing"

func TestIsFullSlugRejectsControlAndWhitespace(t *testing.T) {
	for _, id := range []string{"vendor/mo\ndel", "vendor/mo del", "vendor/mo\tdel", "ven\rdor/model", "vendor/mo\x00del"} {
		if isFullSlug(id) {
			t.Errorf("isFullSlug(%q) = true, want false", id)
		}
	}
	if !isFullSlug("vendor/model-1.5") {
		t.Fatal("a plain vendor/model slug must stay valid")
	}
}

func TestAllowlistRejectsSlugWithNewline(t *testing.T) {
	data := []byte("{\"version\":1,\"harness\":\"opencode\",\"models\":[{\"id\":\"vendor/mo\\ndel\",\"tier\":\"small\",\"max_effort\":\"low\"}]}")
	if _, err := parseOpenRouterAllowlist(data); err == nil {
		t.Fatal("allowlist with a newline in the model id must be rejected")
	}
}
