// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package tools

import (
	"strings"
	"testing"
)

// The providers description is one of the three channels that teach the
// orchestrator to consult the live model list, so it must say to call providers
// before every spawn, not once per session. This golden text is the sensor for
// that guidance (docs/evidence/mcp-claude-code-2026-10-31.md, M6).
func TestProvidersDescriptionAsksToCallBeforeEverySpawn(t *testing.T) {
	for _, want := range []string{"before every subagent spawn", "not once per session", "Agent or Task"} {
		if !strings.Contains(providersDesc, want) {
			t.Errorf("providers description lacks %q: %s", want, providersDesc)
		}
	}
}
