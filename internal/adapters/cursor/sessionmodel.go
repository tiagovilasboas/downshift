// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package cursor

import (
	"os"

	"github.com/tiagovilasboas/downshift-ai/internal/hookutil"
)

// Cursor preToolUse on Task often omits model_id (live 3.27.12). The parent
// conversation transcript is Claude-shaped JSONL under transcript_path; the
// model on the assistant line that issued this tool_use_id is the baseline.

// emptyWait and callWait share the bounded transcript wait with Claude Code.
var emptyWait, callWait hookutil.TranscriptWait

func baselineCurrentModel(ev Event, ti map[string]any) string {
	if model := hookutil.StringField(ti, "model"); model != "" && model != "inherit" {
		return model
	}
	if ev.ModelID != "" {
		return ev.ModelID
	}
	if ev.Model != "" {
		return ev.Model
	}
	if m := recalledComposer(sessionKey(ev)); m != "" {
		return m
	}
	return parentModelAtSpawn(ev.TranscriptPath, ev.ToolUseID)
}

func parentModelAtSpawn(path, toolUseID string) string {
	if toolUseID == "" || path == "" {
		return sessionModelFromTranscript(path)
	}
	if m := hookutil.TranscriptModelOfCall(path, "assistant", "message", toolUseID); m != "" {
		return m
	}
	if _, err := os.Stat(path); err != nil {
		return sessionModelFromTranscript(path)
	}
	if m := callWait.Poll(path, path+"\x00"+toolUseID, func() string {
		return hookutil.TranscriptModelOfCall(path, "assistant", "message", toolUseID)
	}); m != "" {
		return m
	}
	return sessionModelFromTranscript(path)
}

func sessionModelFromTranscript(path string) string {
	if m := hookutil.TranscriptModel(path, "assistant", "message"); m != "" {
		return m
	}
	return emptyWait.Poll(path, path, func() string {
		return hookutil.TranscriptModel(path, "assistant", "message")
	})
}
