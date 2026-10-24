// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package execwrap

import (
	"strings"
	"testing"
)

func TestAllowlistExamplesAreAllAccepted(t *testing.T) {
	for _, example := range allowlistExamples {
		if FamilyOf(strings.Fields(example)) == FamilyNone {
			t.Errorf("example %q is not routed by FamilyOf; fix or drop it", example)
		}
	}
}

func TestAllowlistSummaryIsOneShortLine(t *testing.T) {
	summary := AllowlistSummary()
	if summary == "" || strings.ContainsAny(summary, "\r\n") {
		t.Fatalf("summary must be one non-empty line: %q", summary)
	}
	if len(summary) > 200 {
		t.Errorf("summary is %d bytes, want <= 200: %q", len(summary), summary)
	}
}
