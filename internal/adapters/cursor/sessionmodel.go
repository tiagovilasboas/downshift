// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package cursor

import (
	"os"
	"sync"
	"time"

	"github.com/tiagovilasboas/downshift/internal/hookutil"
)

// Cursor preToolUse on Task often omits model_id (live 3.27.12). The parent
// conversation transcript is Claude-shaped JSONL under transcript_path; the
// model on the assistant line that issued this tool_use_id is the baseline.

var (
	transcriptWaitTotal = 400 * time.Millisecond
	transcriptWaitStep  = 40 * time.Millisecond
)

var waitedEmpty struct {
	sync.Mutex
	path string
}

var callWaited struct {
	sync.Mutex
	path, call string
}

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
	callWaited.Lock()
	again := callWaited.path == path && callWaited.call == toolUseID
	callWaited.Unlock()
	if !again {
		for waited := time.Duration(0); waited < transcriptWaitTotal; waited += transcriptWaitStep {
			time.Sleep(transcriptWaitStep)
			if m := hookutil.TranscriptModelOfCall(path, "assistant", "message", toolUseID); m != "" {
				return m
			}
		}
		callWaited.Lock()
		callWaited.path, callWaited.call = path, toolUseID
		callWaited.Unlock()
	}
	return sessionModelFromTranscript(path)
}

func sessionModelFromTranscript(path string) string {
	if m := hookutil.TranscriptModel(path, "assistant", "message"); m != "" {
		return m
	}
	if path == "" {
		return ""
	}
	if _, err := os.Stat(path); err != nil {
		return ""
	}
	waitedEmpty.Lock()
	again := waitedEmpty.path == path
	waitedEmpty.Unlock()
	if again {
		return ""
	}
	for waited := time.Duration(0); waited < transcriptWaitTotal; waited += transcriptWaitStep {
		time.Sleep(transcriptWaitStep)
		if m := hookutil.TranscriptModel(path, "assistant", "message"); m != "" {
			return m
		}
	}
	waitedEmpty.Lock()
	waitedEmpty.path = path
	waitedEmpty.Unlock()
	return ""
}
