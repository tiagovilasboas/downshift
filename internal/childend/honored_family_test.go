// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package childend

import (
	"testing"

	"github.com/tiagovilasboas/downshift/internal/catalog"
)

// A harness alias can resolve to a newer release of the family the hook
// wrote (live panel: wrote claude-haiku-4-5, ran claude-haiku-5-5). That is
// the same family and tier, so the rewrite is honored.
func TestRewriteHonoredSameFamilyAlias(t *testing.T) {
	r := catalog.Load()
	cases := []struct {
		name, observed, requested string
		known                     bool
		want                      bool
	}{
		{"same id", "claude-haiku-4-5", "claude-haiku-4-5", true, true},
		{"dated id of requested", "claude-haiku-4-5-20251001", "claude-haiku-4-5", true, true},
		{"same family newer version", "claude-haiku-5-5", "claude-haiku-4-5", true, true},
		{"different family", "claude-sonnet-5-5", "claude-haiku-4-5", true, false},
		{"same family different tier", "claude-opus-5-5", "claude-sonnet-5-5", true, false},
		{"unknown observed model", "totally-unknown-9", "claude-haiku-4-5", false, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, okGot := r.LookupByID("claude-code", c.observed)
			if okGot != c.known {
				t.Fatalf("lookup(%q) ok=%v, want %v", c.observed, okGot, c.known)
			}
			if !okGot {
				return // recordResolved leaves RewriteHonored unset: fail closed
			}
			want, ok := r.LookupByID("claude-code", c.requested)
			if !ok {
				t.Fatalf("requested %q not in catalog", c.requested)
			}
			if h := rewriteHonored(got, want); h != c.want {
				t.Fatalf("rewriteHonored(%s, %s) = %v, want %v", got.ID, want.ID, h, c.want)
			}
		})
	}
}
