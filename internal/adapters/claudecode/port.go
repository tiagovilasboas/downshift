// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package claudecode

import (
	"github.com/tiagovilasboas/downshift/internal/core"
	"github.com/tiagovilasboas/downshift/internal/hookport"
)

// Port is Claude Code's honor and usage registration. Honor reports the
// executed child model; usage reports child token usage; ParseChildEnd feeds
// the shared `downshift end claude-code` runner and EndLine wraps its line as a
// systemMessage. None of them chooses a tier. Escalation stays in core.
func Port() hookport.Port {
	return hookport.Port{
		ID: harnessID,
		Honor: func(raw []byte, binaryVersion string, r core.Resolver) (any, string) {
			stdout, note, _ := HandlePostToolUse(raw, binaryVersion, r)
			return stdout, note
		},
		Usage:         HandleSubagentStop,
		UsageOut:      HandleSubagentStopOut,
		ParseChildEnd: ParseChildEnd,
		EndLine:       func(line string) any { return Output{SystemMessage: line} },
	}
}
