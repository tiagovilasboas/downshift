// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package api

import (
	"fmt"
	"os"
	"strings"

	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

// latestSessionHash returns the most recent session hash from the events.
func latestSessionHash(events []telemetry.Event) string {
	return telemetry.LatestSessionHash(events)
}

// envSessionHash checks environment variables from known harnesses.
func envSessionHash() string {
	envVars := []string{
		"CLAUDE_CODE_SESSION_ID",
		"CURSOR_SESSION_ID",
		"CODEX_SESSION_ID",
		"ANTIGRAVITY_SESSION_ID",
	}
	for _, env := range envVars {
		if raw := os.Getenv(env); raw != "" {
			if h := telemetry.HashSessionID(raw); h != "" {
				return h
			}
		}
	}
	return ""
}

// sessionFromID resolves a session ID (raw, hash, or prefix) to a full hash.
func sessionFromID(v string, events []telemetry.Event) (string, error) {
	if telemetry.ValidSessionHash(v) {
		return v, nil
	}
	if h := telemetry.HashSessionID(v); h != "" && !isHexPrefix(v) {
		return h, nil
	}
	if isHexPrefix(v) && len(v) >= 8 {
		found := ""
		for _, ev := range events {
			if telemetry.ValidSessionHash(ev.SessionID) && strings.HasPrefix(ev.SessionID, v) {
				if found != "" && found != ev.SessionID {
					return "", fmt.Errorf("session prefix %q is ambiguous", v)
				}
				found = ev.SessionID
			}
		}
		if found != "" {
			return found, nil
		}
	}
	if h := telemetry.HashSessionID(v); h != "" {
		return h, nil
	}
	return "", fmt.Errorf("invalid session id %q", v)
}

func isHexPrefix(v string) bool {
	if v == "" {
		return false
	}
	for _, c := range v {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return false
		}
	}
	return true
}
