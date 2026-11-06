// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package theme

import (
	"strings"
	"testing"
)

func TestNoColorKeepsSymbolsWithoutColour(t *testing.T) {
	th := build(true)
	for _, s := range []string{"ok", "warn", "fail", "other"} {
		got := th.Status(s)
		if strings.Contains(got, "\x1b[3") {
			t.Errorf("Status(%q) carries a colour escape with NO_COLOR: %q", s, got)
		}
		if strings.TrimSpace(got) == "" {
			t.Errorf("Status(%q) is empty", s)
		}
	}
}
