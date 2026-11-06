// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package hookutil

import (
	"os"
	"sync"
	"time"
)

// TranscriptWaitTotal and TranscriptWaitStep bound how long a hook waits for a
// harness to flush the parent's assistant message to the transcript (variables
// so tests can shorten them).
//
// Claude Code writes the transcript asynchronously. On the first spawn of a
// session the PreToolUse hook can run before the assistant message that
// carries the session model is on disk: in 3 fresh headless runs the
// transcript had no assistant line at the hook in 2 of them, and one 0.5 s
// later. Reading it once then gave an unknown baseline, and an unknown
// baseline holds the rewrite, so those spawns were never routed. 400 ms was
// enough in every measurement and is small next to a hook's own deadline.
var (
	TranscriptWaitTotal = 400 * time.Millisecond
	TranscriptWaitStep  = 40 * time.Millisecond
)

// TranscriptWait remembers the last key whose bounded wait found nothing. One
// hook call reads the transcript twice (telemetry and the decision); the second
// read is still made, but the wait is not paid twice. The zero value is ready.
type TranscriptWait struct {
	mu  sync.Mutex
	key string
}

// Poll re-reads for a short, bounded time and returns the first non-empty
// value. A transcript file that does not exist is not waited for: a session
// run without persistence never writes one, and waiting would only delay every
// spawn. A key that already waited in vain returns "" at once. Fail open: the
// empty result is the unknown answer.
func (w *TranscriptWait) Poll(path, key string, read func() string) string {
	if path == "" {
		return ""
	}
	if _, err := os.Stat(path); err != nil {
		return ""
	}
	w.mu.Lock()
	again := w.key == key
	w.mu.Unlock()
	if again {
		return ""
	}
	for waited := time.Duration(0); waited < TranscriptWaitTotal; waited += TranscriptWaitStep {
		time.Sleep(TranscriptWaitStep)
		if v := read(); v != "" {
			return v
		}
	}
	w.mu.Lock()
	w.key = key
	w.mu.Unlock()
	return ""
}
