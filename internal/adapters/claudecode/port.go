// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package claudecode

import (
	"github.com/tiagovilasboas/downshift-ai/internal/core"
	"github.com/tiagovilasboas/downshift-ai/internal/hookport"
)

// Port is Claude Code's honor and usage registration. Honor reports the
// executed child model; usage reports child token usage; ParseChildEnd feeds
// the shared `downshift end claude-code` runner and EndLine wraps its line as a
// systemMessage. ParseToolOutput and ReplaceToolOutput carry a finished tool's
// result to internal/postrun and its answer back. None of them chooses a tier
// or what to cut. Escalation stays in core.
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
		// The finished tool's result, for the shared post-run compaction.
		ParseToolOutput:   ParseToolOutput,
		ReplaceToolOutput: ReplaceToolOutput,
	}
}
