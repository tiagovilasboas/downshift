// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

// Package quota supplies offline budget evidence for harness-downshift by
// Tiago de Carvalho Vilas Boas, https://github.com/tiagovilasboas/downshift.
package quota

import (
	"encoding/json"
	"errors"
	"math"
	"os"
	"path/filepath"
	"time"

	"github.com/tiagovilasboas/downshift/internal/paths"
)

type Status string

const (
	MaxTTL           = 15 * time.Minute
	Available Status = "available"
	Exhausted Status = "exhausted"
	Unknown   Status = "unknown"
	Stale     Status = "stale"
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

func Path() (string, error) {
	if p := os.Getenv("DOWNSHIFT_QUOTA_FILE"); p != "" {
		return p, nil
	}
	return paths.Join("quota.json")
}

type File struct {
	Version   int                 `json:"version"`
	Harnesses map[string]Snapshot `json:"harnesses"`
}

// Load distinguishes missing evidence from malformed evidence; neither
// case permits a quota claim. No network or credential access occurs here.
func Load(harness string) (*Snapshot, bool) {
	p, err := Path()
	if err != nil {
		return nil, false
	}
	data, err := os.ReadFile(p)
	if os.IsNotExist(err) {
		return nil, false
	}
	if err != nil {
		return nil, true
	}
	var f File
	if json.Unmarshal(data, &f) != nil || f.Version != 1 || f.Harnesses == nil {
		return nil, true
	}
	s, ok := f.Harnesses[harness]
	if !ok {
		return nil, true
	}
	return &s, true
}

// Store atomically replaces one harness's snapshot, preserving the others.
func Store(path string, s Snapshot) error {
	if err := s.Validate(); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	// Directory lock is portable and never used by the hook reader. A crashed
	// writer leaves a visible lock rather than silently losing another harness.
	deadline := time.Now().Add(time.Second)
	for {
		if err := os.Mkdir(path+".lock", 0700); err == nil {
			break
		} else if !os.IsExist(err) {
			return err
		}
		if time.Now().After(deadline) {
			return errors.New("quota cache busy; retry or inspect abandoned .lock directory")
		}
		time.Sleep(10 * time.Millisecond)
	}
	defer os.Remove(path + ".lock")
	f := File{Version: 1, Harnesses: map[string]Snapshot{}}
	if b, err := os.ReadFile(path); err == nil {
		if json.Unmarshal(b, &f) != nil || f.Version != 1 || f.Harnesses == nil {
			return errors.New("invalid quota cache; refusing to overwrite")
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	if previous, exists := f.Harnesses[s.Harness]; exists && previous.ObservedAt.After(s.ObservedAt) {
		return errors.New("quota observation is older than the cached observation")
	}
	f.Harnesses[s.Harness] = s
	b, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".quota-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(append(b, '\n')); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}
