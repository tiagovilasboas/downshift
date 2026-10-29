// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

// Package quotamodel is the filesystem-free quota evidence model.
//
// Downshift by Tiago Vilas Boas
// https://github.com/tiagovilasboas/downshift
package quotamodel

import (
	"errors"
	"math"
	"time"
)

type Status string

const (
	MaxTTL = 15 * time.Minute
	// StaleGrace is how long a valid snapshot keeps holding rewrites after
	// expires_at. Past it the snapshot is treated as absent (see Abandoned).
	StaleGrace        = time.Hour
	Available  Status = "available"
	Exhausted  Status = "exhausted"
	Unknown    Status = "unknown"
	Stale      Status = "stale"
)

// Window is a provider-reported budget, never a catalog or session mark.
// Scope harness is shared across every model of that harness. Scope models
// applies only to the exact IDs the provider associated with this pool.
type Window struct {
	ID          string    `json:"id"`
	Scope       string    `json:"scope"`
	ModelIDs    []string  `json:"model_ids,omitempty"`
	UsedPercent *float64  `json:"used_percent,omitempty"`
	ResetsAt    time.Time `json:"resets_at"`
}

type Snapshot struct {
	Version    int       `json:"version"`
	Harness    string    `json:"harness"`
	Source     string    `json:"source"`
	ObservedAt time.Time `json:"observed_at"`
	ExpiresAt  time.Time `json:"expires_at"`
	Windows    []Window  `json:"windows"`
}

// Validate checks structure; Evaluate additionally checks live freshness.
func (s Snapshot) Validate() error {
	if s.Version != 1 || s.Harness == "" || !validSource(s.Source) || s.ObservedAt.IsZero() || !s.ExpiresAt.After(s.ObservedAt) || s.ExpiresAt.Sub(s.ObservedAt) > MaxTTL {
		return errors.New("quota requires version 1, harness, source and an observation expiry")
	}
	seen := map[string]bool{}
	for _, w := range s.Windows {
		if w.ID == "" || seen[w.ID] || (w.Scope != "harness" && w.Scope != "models") || (w.Scope == "models" && len(w.ModelIDs) == 0) || (w.Scope == "harness" && len(w.ModelIDs) > 0) {
			return errors.New("quota windows require unique IDs and explicit harness or model coverage")
		}
		seen[w.ID] = true
		if w.UsedPercent != nil && (math.IsNaN(*w.UsedPercent) || math.IsInf(*w.UsedPercent, 0) || *w.UsedPercent < 0 || *w.UsedPercent > 100) {
			return errors.New("quota used_percent must be between 0 and 100")
		}
	}
	return nil
}

// Abandoned reports a structurally valid snapshot that expired more than
// StaleGrace ago. Callers treat it as absent evidence unless the operator
// opted into DOWNSHIFT_QUOTA_MODE=required. A malformed snapshot is never
// abandoned: it keeps holding.
func (s Snapshot) Abandoned(now time.Time) bool {
	return s.Validate() == nil && now.After(s.ExpiresAt.Add(StaleGrace))
}

// StaleAfterObserved reports a reading observed more than MaxTTL ago, the
// longest lifetime any snapshot may claim. Exactly at MaxTTL is still fresh.
// It is a freshness signal for operators; the gate keeps using ExpiresAt.
func (s Snapshot) StaleAfterObserved(now time.Time) bool {
	return now.Sub(s.ObservedAt) > MaxTTL
}

func validSource(source string) bool {
	switch source {
	case "codex-usage", "codex-transcript", "claude-statusline", "operator-normalized", "cursor-usage":
		return true
	default:
		return false
	}
}

// SourceName is the fixed provenance vocabulary safe for prompt-free logs.
func SourceName(source string) string {
	if validSource(source) {
		return source
	}
	return ""
}

// Evaluate intersects all applicable windows. A shared exhausted window
// blocks every covered model, without inventing a balance per model.
func (s Snapshot) Evaluate(harness, model string, now time.Time) Status {
	if s.Harness != harness || s.Validate() != nil || s.ObservedAt.After(now) {
		return Unknown
	}
	if !now.Before(s.ExpiresAt) {
		return Stale
	}
	matched, unknown, stale, exhausted := false, false, false, false
	for _, w := range s.Windows {
		if !w.covers(model) {
			continue
		}
		matched = true
		if w.UsedPercent == nil {
			unknown = true
			continue
		}
		if w.ResetsAt.IsZero() {
			unknown = true
			continue
		}
		if !now.Before(w.ResetsAt) {
			stale = true
			continue
		}
		if *w.UsedPercent >= 100 {
			exhausted = true
		}
	}
	if exhausted {
		return Exhausted
	}
	if stale {
		return Stale
	}
	if !matched || unknown {
		return Unknown
	}
	return Available
}

func (w Window) covers(model string) bool {
	if w.Scope == "harness" {
		return true
	}
	for _, id := range w.ModelIDs {
		if id != "" && id == model {
			return true
		}
	}
	return false
}
