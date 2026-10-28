// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

// Package cascade implements sensor-guided auto-upgrade and episodic failure
// tracking (dead-ends). When a test or subagent execution fails on an economic
// model, cascade records the failure so subsequent spawns automatically escalate
// (upshift) to a higher tier instead of looping on a failed model.
package cascade

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/tiagovilasboas/downshift/internal/paths"
)

// DeadEnd records one failed subagent execution or test run.
type DeadEnd struct {
	SessionID   string    `json:"session_id"`
	Fingerprint string    `json:"fingerprint"` // SHA256 of task description or command
	FailedModel string    `json:"failed_model,omitempty"`
	Reason      string    `json:"reason"`
	Timestamp   time.Time `json:"timestamp"`
}

var (
	mu sync.RWMutex
)

// TaskFingerprint computes a 16-hex deterministic hash of a task or command.
func TaskFingerprint(text string) string {
	cleaned := strings.TrimSpace(strings.ToLower(text))
	if cleaned == "" {
		return ""
	}
	h := sha256.Sum256([]byte(cleaned))
	return hex.EncodeToString(h[:8])
}

// deadEndsPath returns the path to dead_ends.json in the state directory.
func deadEndsPath() (string, error) {
	return paths.Join("dead_ends.json")
}

// RecordFailure records a failure event in the dead-ends store.
// Thread-safe and fail-open.
func RecordFailure(sessionID, taskText, failedModel, reason string) {
	fp := TaskFingerprint(taskText)
	if sessionID == "" && fp == "" {
		return
	}
	p, err := deadEndsPath()
	if err != nil {
		return
	}

	mu.Lock()
	defer mu.Unlock()

	entries := loadEntries(p)
	// Deduplicate / update existing failure for the same fingerprint and session
	now := time.Now().UTC()
	updated := false
	for i := range entries {
		if entries[i].SessionID == sessionID && entries[i].Fingerprint == fp {
			entries[i].FailedModel = failedModel
			entries[i].Reason = reason
			entries[i].Timestamp = now
			updated = true
			break
		}
	}
	if !updated {
		entries = append(entries, DeadEnd{
			SessionID:   sessionID,
			Fingerprint: fp,
			FailedModel: failedModel,
			Reason:      reason,
			Timestamp:   now,
		})
	}

	// Keep only the last 100 entries to prevent unbounded growth
	if len(entries) > 100 {
		entries = entries[len(entries)-100:]
	}

	saveEntries(p, entries)
}

// HasRecentFailure checks if a matching task or session failed within the given duration.
func HasRecentFailure(sessionID, taskText string, window time.Duration) (DeadEnd, bool) {
	fp := TaskFingerprint(taskText)
	p, err := deadEndsPath()
	if err != nil {
		return DeadEnd{}, false
	}

	mu.RLock()
	defer mu.RUnlock()

	entries := loadEntries(p)
	cutoff := time.Now().UTC().Add(-window)

	for i := len(entries) - 1; i >= 0; i-- {
		e := entries[i]
		if e.Timestamp.Before(cutoff) {
			continue
		}
		if sessionID != "" && e.SessionID == sessionID {
			if fp == "" || e.Fingerprint == fp {
				return e, true
			}
		} else if fp != "" && e.Fingerprint == fp {
			return e, true
		}
	}
	return DeadEnd{}, false
}

// ClearSession removes failure records for a given session.
func ClearSession(sessionID string) {
	if sessionID == "" {
		return
	}
	p, err := deadEndsPath()
	if err != nil {
		return
	}

	mu.Lock()
	defer mu.Unlock()

	entries := loadEntries(p)
	filtered := entries[:0]
	for _, e := range entries {
		if e.SessionID != sessionID {
			filtered = append(filtered, e)
		}
	}
	saveEntries(p, filtered)
}

func loadEntries(path string) []DeadEnd {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var entries []DeadEnd
	_ = json.Unmarshal(data, &entries)
	return entries
}

func saveEntries(path string, entries []DeadEnd) {
	data, err := json.Marshal(entries)
	if err != nil {
		return
	}
	_ = os.WriteFile(path, data, 0o600)
}
