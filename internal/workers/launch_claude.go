// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package workers

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"fmt"
)

// newUUIDv4 generates a random version-4 UUID from the crypto RNG. It is the id
// the Claude launcher hands the CLI on --session-id so the Downshift knows the
// session before the process starts: a cancel on the very first turn, which
// SIGKILLs the CLI before it writes its final JSON, still has an id to delete
// the transcript with. A random 128-bit id makes a collision between two
// workers' sessions not a practical concern. It never fails in practice;
// crypto/rand.Read only errors when the OS RNG is unavailable, and the one
// caller treats an empty id as "no preset" and falls back to capture-after-run.
func newUUIDv4() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return ""
	}
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant 10
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

func claudeReadOnly(spec Spec) (argv, error) {
	return claudeArgv("plan", spec), nil
}

// claudeArgv builds the Claude recipe shared by the read-only and write modes,
// differing only in the permission mode. It generates the session id upfront
// (newUUIDv4) and passes it on --session-id, so the Downshift knows the id
// before the process starts; the same id is kept on the argv as presetSession
// so the launcher can report it to the queue at start. An empty id (the RNG was
// unavailable) simply omits the flag and leaves the recipe to capture the id
// the CLI writes after the run, the previous behaviour.
func claudeArgv(mode string, spec Spec) argv {
	args := claudeModeArgs(mode)
	id := newUUIDv4()
	if id != "" {
		args = append(args, "--session-id", id)
	}
	args = appendClaudeTuning(args, spec)
	return argv{name: "claude", args: args, presetSession: id, report: claudeReport, tokens: claudeTokens, session: claudeSession, resume: func(sessionID string) []string { return claudeResumeArgs(mode, spec, sessionID) }, cleanSession: claudeCleanSession}
}

// appendClaudeTuning adds the model and effort flags a spec carries, shared by
// the read-only and write recipes so both stay identical but for the permission
// mode.
func appendClaudeTuning(args []string, spec Spec) []string {
	if spec.Model != "" {
		args = append(args, "--model", spec.Model)
	}
	if spec.Effort != "" {
		args = append(args, "--effort", spec.Effort)
	}
	return args
}

// claudeResumeArgs are the args that continue a stored claude session with a
// new turn: `claude -p --output-format json` plus the same isolation, the same
// permission mode, model and effort as the first launch (a resume never climbs
// a tier or widens a mode, and never silently drops to the CLI defaults). The
// id goes last so a malformed id cannot shift a flag.
func claudeResumeArgs(mode string, spec Spec, sessionID string) []string {
	args := appendClaudeTuning(claudeModeArgs(mode), spec)
	return append(args, "--resume", sessionID)
}

// claudeReport reads the single JSON object print mode writes: its result text,
// or the reason it did not succeed.
func claudeReport(stdout []byte) (string, error) {
	var r struct {
		Result  string `json:"result"`
		IsError bool   `json:"is_error"`
		Subtype string `json:"subtype"`
	}
	if err := json.Unmarshal(stdout, &r); err != nil {
		return "", fmt.Errorf("claude wrote no JSON result: %v", err)
	}
	if r.IsError || r.Subtype != "success" {
		return r.Result, fmt.Errorf("claude ended with %q", r.Subtype)
	}
	return r.Result, nil
}

// claudeSession reads the session id from the single JSON object print mode
// writes (the `session_id` field, confirmed in the installed CLI 2.1.316). It
// returns "" when the output carries no id (an older CLI, an error, or a
// malformed line), so a worker without a captured id simply is not resumable
// and nothing breaks.
func claudeSession(stdout []byte) string {
	var r struct {
		SessionID string `json:"session_id"`
	}
	if json.Unmarshal(bytes.TrimSpace(stdout), &r) != nil {
		return ""
	}
	return r.SessionID
}

// claudeWrite is the write recipe: the bypass permission mode, inside a worktree
// that writeIsolated prepared.
func claudeWrite(spec Spec) (argv, error) {
	return claudeArgv("bypassPermissions", spec), nil
}
