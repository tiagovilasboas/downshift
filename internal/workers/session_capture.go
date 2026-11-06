// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package workers

// Capturing a worker's session id is what lets a later turn resume it and what
// lets the queue delete its transcript when it is done (release, cancel, close).
// There are two moments to capture it, and this file holds the one the queue
// shares between starting a first turn and resuming a later one:
//
//   - upfront, at start: a launcher that set the id with the CLI before the run
//     (Claude's generated --session-id) reports it through presetSessionReporter,
//     so the id is on the job the instant the worker starts. This is what closes
//     the first-turn-cancel gap: a worker SIGKILLed before its CLI writes any
//     output still has an id to clean up by.
//   - after the run, in wait/finish: a launcher whose CLI assigns the id only at
//     runtime (Codex's thread id) reports it through sessionReporter once the
//     process has flushed its output.
//
// The queue stays agnostic in both: it reads an opaque id through an optional
// interface and never learns which agent produced it.

// recordPresetSessionLocked stores the session id the launcher knew before the
// process started, when the process reports one. It is a no-op for a process
// whose CLI assigns the id only at runtime (it reports no preset), leaving the
// id to be captured from the output after the worker exits. The caller holds
// the queue lock. It is called from both start (a first turn) and resumeNext (a
// later turn), so a worker cancelled on either has an id to clean up by.
func (q *Queue) recordPresetSessionLocked(j *job, proc Process) {
	pr, ok := proc.(presetSessionReporter)
	if !ok {
		return
	}
	if id := pr.PresetSessionID(); id != "" {
		j.sessionID = id
	}
}
