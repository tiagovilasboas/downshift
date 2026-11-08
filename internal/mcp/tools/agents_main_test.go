// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package tools

import (
	"os"
	"testing"

	"github.com/tiagovilasboas/downshift-ai/internal/workers"
)

// TestMain installs the agent to harness map the binary builds from the
// harness table, which these tests cannot import.
func TestMain(m *testing.M) {
	workers.SetAgentHarnesses(map[string]string{"claude": "claude-code", "codex": "codex"})
	os.Exit(m.Run())
}
