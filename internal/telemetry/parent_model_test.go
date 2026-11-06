// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package telemetry_test

import (
	"testing"

	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

func TestParentModelLabel(t *testing.T) {
	cases := []struct{ model, source, wantModel, wantSource string }{
		{"claude-opus-4-7", "transcript", "claude-opus-4-7", "transcript"},
		{"claude-opus-4-7", "Bad Source!", "claude-opus-4-7", ""},
		{"", "mod", "", ""},
		{"free text with spaces", "mod", "", ""},
	}
	for _, c := range cases {
		m, s := telemetry.ParentModelLabel(c.model, c.source)
		if m != c.wantModel || s != c.wantSource {
			t.Errorf("ParentModelLabel(%q, %q) = %q, %q; want %q, %q", c.model, c.source, m, s, c.wantModel, c.wantSource)
		}
	}
}
