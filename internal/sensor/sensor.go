// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

// Package sensor provides a lightweight, event-driven context sensor.
// It runs in-process with zero background daemons or continuous polling.
// It strictly distinguishes between:
//   - Observed: exact values supplied by the harness (e.g. tool execution count, reported tokens).
//   - Estimated: local heuristics (e.g. estimated tokens from bytes reduced).
//   - Unavailable: telemetry data the harness does not supply.
//
// Prompts, private code, and sensitive responses are never stored.
package sensor

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/tiagovilasboas/downshift/internal/compressor"
	"github.com/tiagovilasboas/downshift/internal/paths"
)

// MetricState defines the confidence level of a reported metric.
type MetricState string

const (
	StateObserved    MetricState = "observed"
	StateEstimated   MetricState = "estimated"
	StateUnavailable MetricState = "unavailable"
)

// MetricValue pairs a numeric or string value with its confidence state.
type MetricValue struct {
	State MetricState `json:"state"`
	Value any         `json:"value,omitempty"`
}

// SessionObservation aggregates context metrics for a session without storing raw text.
type SessionObservation struct {
	SessionHash              string                      `json:"session_hash"`
	Harness                  string                      `json:"harness"`
	LastSeen                 time.Time                   `json:"last_seen"`
	ToolExecutions           int64                       `json:"tool_executions"`
	FailureCount             int64                       `json:"failure_count"`
	TotalOutputBytes         int64                       `json:"total_output_bytes"`
	PotentialReducedBytes    int64                       `json:"potential_reduced_bytes"`
	CompressionOpportunities int64                       `json:"compression_opportunities"`
	FormatBreakdown          map[compressor.Format]int64 `json:"format_breakdown"`
	InputTokens              MetricValue                 `json:"input_tokens"`
	OutputTokens             MetricValue                 `json:"output_tokens"`
}

// Store persists aggregated context sensor observations safely under the state dir.
type Store struct {
	mu   sync.Mutex
	path string
}

// DefaultStore returns the sensor store at $DOWNSHIFT_STATE_DIR/context-sensor.json.
func DefaultStore() (*Store, error) {
	p, err := paths.Join("context-sensor.json")
	if err != nil {
		return nil, err
	}
	return &Store{path: p}, nil
}

// RecordToolOutput observes a tool output event safely without storing the content.
func (s *Store) RecordToolOutput(sessionHash, harness, toolName string, output []byte, isError bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := s.loadAll()
	if err != nil {
		data = make(map[string]SessionObservation)
	}

	obs, exists := data[sessionHash]
	if !exists {
		obs = SessionObservation{
			SessionHash:     sessionHash,
			Harness:         harness,
			FormatBreakdown: make(map[compressor.Format]int64),
			InputTokens:     MetricValue{State: StateUnavailable},
			OutputTokens:    MetricValue{State: StateUnavailable},
		}
	}

	obs.LastSeen = time.Now().UTC()
	obs.ToolExecutions++
	if isError {
		obs.FailureCount++
	}
	obs.TotalOutputBytes += int64(len(output))

	// Run compressor in observe mode to measure compression potential safely
	res := compressor.Compress(output, compressor.ModeObserve)
	if res.Applied {
		obs.CompressionOpportunities++
		obs.PotentialReducedBytes += int64(res.OriginalBytes - res.ReducedBytes)
		if obs.FormatBreakdown == nil {
			obs.FormatBreakdown = make(map[compressor.Format]int64)
		}
		obs.FormatBreakdown[res.Format]++
	}

	data[sessionHash] = obs
	return s.saveAll(data)
}

// GetSummary returns all session observations without raw content.
func (s *Store) GetSummary() (map[string]SessionObservation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.loadAll()
}

func (s *Store) loadAll() (map[string]SessionObservation, error) {
	raw, err := os.ReadFile(s.path)
	if err != nil {
		return make(map[string]SessionObservation), err
	}
	var res map[string]SessionObservation
	if err := json.Unmarshal(raw, &res); err != nil {
		return make(map[string]SessionObservation), err
	}
	return res, nil
}

func (s *Store) saveAll(data map[string]SessionObservation) error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0755); err != nil {
		return err
	}
	bytes, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, bytes, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}
