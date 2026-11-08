// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"time"

	"github.com/tiagovilasboas/downshift-ai/internal/telemetry"
)

// ── event ─────────────────────────────────────────────────────────────────────

// event is one event-log line. It reuses the telemetry schema instead of
// keeping a private copy of it.
type event struct {
	telemetry.Event
}

func (e event) localTime() string {
	t, err := time.Parse(time.RFC3339Nano, e.Timestamp)
	if err != nil || len(e.Timestamp) < 16 {
		return "??:??"
	}
	// Show relative time for recent events so the list feels live
	age := time.Since(t)
	switch {
	case age < 60*time.Second:
		return fmt.Sprintf("%2ds", int(age.Seconds()))
	case age < 3600*time.Second:
		return fmt.Sprintf("%2dm", int(age.Minutes()))
	default:
		return t.Local().Format("15:04")
	}
}

// isRecent returns true for events within the last 24 hours.
// Using 24h instead of "today" avoids timezone edge cases and midnight cutoffs.
func (e event) isRecent() bool {
	t, err := time.Parse(time.RFC3339Nano, e.Timestamp)
	if err != nil {
		return false
	}
	return time.Since(t) < window24h
}

// ── agent event (agents.jsonl from dsmon-hook) ────────────────────────────────

type agentEvent struct {
	Timestamp string `json:"timestamp"`
	Tool      string `json:"tool"`
	Session   string `json:"session"`
	Task      string `json:"task"`
	Model     string `json:"model"`
}

func (a agentEvent) isRecent() bool {
	t, err := time.Parse(time.RFC3339Nano, a.Timestamp)
	return err == nil && time.Since(t) < window24h
}

func (a agentEvent) age() string {
	t, err := time.Parse(time.RFC3339Nano, a.Timestamp)
	if err != nil {
		return "?"
	}
	d := time.Since(t)
	switch {
	case d < 60*time.Second:
		return fmt.Sprintf("%2ds", int(d.Seconds()))
	case d < 3600*time.Second:
		return fmt.Sprintf("%2dm", int(d.Minutes()))
	default:
		return t.Local().Format("15:04")
	}
}
