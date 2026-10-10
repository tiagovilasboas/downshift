// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package claudecode

import (
	"github.com/tiagovilasboas/downshift/internal/core"
	"github.com/tiagovilasboas/downshift/internal/hookport"
)

// Port is Claude Code's honor and usage registration. Honor reports the
// executed child model; usage reports child token usage. Neither func chooses
// a tier. Escalation stays in core.
func Port() hookport.Port {
	return hookport.Port{
		ID: harnessID,
		Honor: func(raw []byte, binaryVersion string, r core.Resolver) (any, string) {
			stdout, note, _ := HandlePostToolUse(raw, binaryVersion, r)
			return stdout, note
		},
		Usage: HandleSubagentStop,
	}
}
