// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package quota

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"time"
)

const transcriptTailBytes = 2 << 20

// CollectCodex reads only the bounded tail of an explicitly supplied local
// transcript. It preserves the event timestamp; reading an old file cannot
// renew a budget. Prompts, responses and credits are never retained.
func CollectCodex(path string, ttl time.Duration) (Snapshot, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return Snapshot{}, err
	}
	if !info.Mode().IsRegular() {
		return Snapshot{}, errors.New("transcript must be a regular file, not a pipe or symlink")
	}
	f, err := os.Open(path)
	if err != nil {
		return Snapshot{}, err
	}
	defer f.Close()
	info, err = f.Stat()
	if err != nil {
		return Snapshot{}, err
	}
	if !info.Mode().IsRegular() {
		return Snapshot{}, errors.New("transcript must be a regular file")
	}
	start := info.Size() - transcriptTailBytes
	if start < 0 {
		start = 0
	}
	if _, err := f.Seek(start, io.SeekStart); err != nil {
		return Snapshot{}, err
	}
	b, err := io.ReadAll(io.LimitReader(f, transcriptTailBytes))
	if err != nil {
		return Snapshot{}, err
	}
	lines := bytes.Split(b, []byte{'\n'})
	if start > 0 && len(lines) > 0 {
		lines = lines[1:]
	}
	var latest Snapshot
	for _, line := range lines {
		var event struct {
			Timestamp time.Time `json:"timestamp"`
			Type      string    `json:"type"`
			Payload   struct {
				Type   string          `json:"type"`
				Limits json.RawMessage `json:"rate_limits"`
			} `json:"payload"`
		}
		if json.Unmarshal(line, &event) != nil || event.Type != "event_msg" || event.Payload.Type != "token_count" || len(event.Payload.Limits) == 0 || event.Timestamp.IsZero() {
			continue
		}
		var l struct {
			ID      string `json:"limit_id"`
			Primary *struct {
				Used  *float64 `json:"used_percent"`
				Reset int64    `json:"resets_at"`
			} `json:"primary"`
			Secondary *struct {
				Used  *float64 `json:"used_percent"`
				Reset int64    `json:"resets_at"`
			} `json:"secondary"`
		}
		decodeErr := json.Unmarshal(event.Payload.Limits, &l)
		if decodeErr == nil && l.ID != "" && l.ID != "codex" {
			continue
		}
		s := Snapshot{Version: 1, Harness: "codex", Source: "codex-transcript", ObservedAt: event.Timestamp, ExpiresAt: event.Timestamp.Add(ttl)}
		if l.Primary != nil {
			s.Windows = append(s.Windows, window("codex/primary", "harness", nil, l.Primary.Used, l.Primary.Reset))
		} else {
			s.Windows = append(s.Windows, window("codex/primary", "harness", nil, nil, 0))
		}
		if l.Secondary != nil {
			s.Windows = append(s.Windows, window("codex/secondary", "harness", nil, l.Secondary.Used, l.Secondary.Reset))
		} else {
			s.Windows = append(s.Windows, window("codex/secondary", "harness", nil, nil, 0))
		}
		if decodeErr != nil || l.ID == "" || s.Validate() != nil {
			// Keep the newer observation as UNKNOWN. Falling back to an older
			// healthy event would contradict the latest provider evidence.
			s.Windows = nil
		}
		if latest.ObservedAt.IsZero() || !s.ObservedAt.Before(latest.ObservedAt) {
			latest = s
		}
	}
	if latest.ObservedAt.IsZero() {
		return latest, errors.New("no Codex subscription quota event in transcript tail")
	}
	return latest, nil
}
