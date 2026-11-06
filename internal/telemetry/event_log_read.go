// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package telemetry

import (
	"bufio"
	"encoding/json"
	"io"
	"os"
	"time"
)

// ReadEvents reads all events from the default event log. Returns an empty
// slice (not an error) when the file does not exist.
func ReadEvents() ([]Event, error) {
	return ReadEventsFrom(defaultEventPath())
}

// ReadEventsFrom reads all events from path.
func ReadEventsFrom(path string) ([]Event, error) {
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return parseEvents(f)
}

func parseEvents(r io.Reader) ([]Event, error) {
	var events []Event
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), MaxLineBytes)
	for sc.Scan() {
		line := sc.Bytes()
		if len(line) == 0 {
			continue
		}
		var ev Event
		if err := json.Unmarshal(line, &ev); err != nil {
			continue // skip malformed lines — never block on corrupted log
		}
		events = append(events, ev)
	}
	linkLateUsage(events)
	return events, sc.Err()
}

// FilterByDays returns events within the last n days.
// When n <= 0, all events are returned.
func FilterByDays(events []Event, days int) []Event {
	if days <= 0 {
		return events
	}
	cutoff := time.Now().UTC().Add(-time.Duration(days) * 24 * time.Hour)
	var out []Event
	for _, ev := range events {
		// Events are written with time.RFC3339Nano (FromDecision, Failure);
		// time.RFC3339 cannot parse fractional seconds, so try the nano
		// layout first and accept the legacy layout for older log lines.
		// A timestamp that parses in neither layout is excluded
		// (fail-closed for windowing: a dateless event belongs to no
		// window, and silently including it would inflate every window).
		t, err := time.Parse(time.RFC3339Nano, ev.Timestamp)
		if err != nil {
			t, err = time.Parse(time.RFC3339, ev.Timestamp)
		}
		if err != nil {
			continue
		}
		if t.After(cutoff) {
			out = append(out, ev)
		}
	}
	return out
}
