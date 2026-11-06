// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package workers

import (
	"os"
	"testing"
)

// TestMain installs the agent to harness map the binary builds from the
// harness table, which these tests cannot import.
func TestMain(m *testing.M) {
	SetAgentHarnesses(map[string]string{"claude": "claude-code", "codex": "codex"})
	os.Exit(m.Run())
}
