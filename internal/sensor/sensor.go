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
	"fmt"
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
	mu             sync.Mutex
	path           string
	compactionPath string
}

// DefaultStore returns the sensor store at $DOWNSHIFT_STATE_DIR/context-sensor.json.
// The append-only history is context-compactions.jsonl in the same state dir.
func DefaultStore() (*Store, error) {
	p, err := paths.Join("context-sensor.json")
	if err != nil {
		return nil, err
	}
	logPath, err := paths.Join(compactionLogName)
	if err != nil {
		return nil, err
	}
	return &Store{path: p, compactionPath: logPath}, nil
}

// RecordToolOutput observes a tool output event safely without storing the content.
// The exit code is unknown. A text heuristic may count compression potential,
// and a non-zero status passed via RecordToolResult never does.
func (s *Store) RecordToolOutput(sessionHash, harness, toolName string, output []byte, isError bool) error {
	return s.RecordToolResult(sessionHash, harness, toolName, output, -1, isError)
}

// RecordToolResult is RecordToolOutput with the process exit code.
// exitCode < 0 means unknown. exitCode > 0, or isError, preserves the output
// and does not count a compression opportunity.
func (s *Store) RecordToolResult(sessionHash, harness, toolName string, output []byte, exitCode int, isError bool) error {
	if exitCode > 0 {
		isError = true
	} else if isError && exitCode <= 0 {
		exitCode = 1
	}
	return s.withStateLock(func() error {
		s.mu.Lock()
		defer s.mu.Unlock()
		return s.recordLocked(sessionHash, harness, toolName, output, exitCode, isError)
	})
}

func (s *Store) recordLocked(sessionHash, harness, toolName string, output []byte, exitCode int, isError bool) error {
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

	// Observe mode measures potential only. A known failure is not potential.
	res := compressor.CompressExit(output, compressor.ModeObserve, exitCode)
	if res.Applied {
		obs.CompressionOpportunities++
		obs.PotentialReducedBytes += int64(res.OriginalBytes - res.ReducedBytes)
		if obs.FormatBreakdown == nil {
			obs.FormatBreakdown = make(map[compressor.Format]int64)
		}
		obs.FormatBreakdown[res.Format]++
	}

	data[sessionHash] = obs
	if err := s.saveAll(data); err != nil {
		return err
	}
	// History is append-only and grouped by harness. It never stores the
	// tool output that the aggregate above already measured.
	return s.appendCompaction(newCompactionRecord(sessionHash, harness, toolName, output, isError, res, obs.LastSeen))
}

// CompactionSummary is the tool-output reduction the dashboard may show.
// Bytes are observed counts. Token counts stay zero and token_state stays
// unavailable: this package does not apply a bytes-to-token divisor.
type CompactionSummary struct {
	Sessions       int     `json:"sessions"`
	ToolExecutions int64   `json:"tool_executions"`
	BytesBefore    int64   `json:"bytes_before"`
	BytesReduced   int64   `json:"bytes_reduced"`
	BytesAfter     int64   `json:"bytes_after"`
	TokensBefore   int64   `json:"tokens_before"`
	TokensAfter    int64   `json:"tokens_after"`
	TokensReduced  int64   `json:"tokens_reduced"`
	TokenState     string  `json:"token_state"`
	SavingsPct     float64 `json:"savings_pct"`
	State          string  `json:"state"`
}

// SummarizeCompaction adds tool-output bytes. A session key that is not a
// 64-hex digest fails the whole summary closed. It does not multiply by
// later turns: the sensor does not record how many times history was reread.
func SummarizeCompaction(obs map[string]SessionObservation) CompactionSummary {
	sum := CompactionSummary{
		State:      string(StateUnavailable),
		TokenState: string(StateUnavailable),
	}
	if len(obs) == 0 {
		return sum
	}
	for key := range obs {
		if !sessionDigest(key) {
			return CompactionSummary{
				State:      string(StateUnavailable),
				TokenState: string(StateUnavailable),
			}
		}
	}
	sum.Sessions = len(obs)
	for _, o := range obs {
		sum.ToolExecutions += o.ToolExecutions
		sum.BytesBefore += o.TotalOutputBytes
		sum.BytesReduced += o.PotentialReducedBytes
	}
	if sum.BytesReduced > sum.BytesBefore {
		sum.BytesReduced = sum.BytesBefore
	}
	sum.BytesAfter = sum.BytesBefore - sum.BytesReduced
	if sum.BytesBefore > 0 {
		sum.SavingsPct = float64(sum.BytesReduced) / float64(sum.BytesBefore) * 100
	}
	sum.State = string(StateObserved)
	return sum
}

func sessionDigest(key string) bool {
	if len(key) != 64 {
		return false
	}
	for _, c := range key {
		switch {
		case c >= '0' && c <= '9':
		case c >= 'a' && c <= 'f':
		default:
			return false
		}
	}
	return true
}

// GetSummary returns all session observations without raw content.
func (s *Store) GetSummary() (map[string]SessionObservation, error) {
	var out map[string]SessionObservation
	err := s.withStateLock(func() error {
		s.mu.Lock()
		defer s.mu.Unlock()
		var loadErr error
		out, loadErr = s.loadAll()
		return loadErr
	})
	return out, err
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
	if err := os.MkdirAll(filepath.Dir(s.path), 0700); err != nil {
		return err
	}
	payload, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	tmp := fmt.Sprintf("%s.%d.%d.tmp", s.path, os.Getpid(), time.Now().UnixNano())
	if err := os.WriteFile(tmp, payload, 0600); err != nil {
		return err
	}
	if err := os.Rename(tmp, s.path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

func (s *Store) withStateLock(fn func() error) error {
	if s.path == "" {
		return fmt.Errorf("sensor store path is empty")
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0700); err != nil {
		return err
	}
	f, err := os.OpenFile(s.path+".lock", os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return err
	}
	defer f.Close()
	if err := lockFile(f); err != nil {
		return err
	}
	defer unlockFile(f)
	return fn()
}
