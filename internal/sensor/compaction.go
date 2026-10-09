// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package sensor

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/tiagovilasboas/downshift/internal/compressor"
	"github.com/tiagovilasboas/downshift/internal/paths"
)

const compactionMaxAge = 90 * 24 * time.Hour

// compactionMaxRecords is the newest-record cap. Tests may lower it.
var compactionMaxRecords = 5000

// HarnessCompaction is the byte aggregate for one harness. Tokens stay unavailable.
type HarnessCompaction struct {
	Harness       string  `json:"harness"`
	TotalCommands int64   `json:"total_commands"`
	BytesBefore   int64   `json:"bytes_before"`
	BytesAfter    int64   `json:"bytes_after"`
	BytesReduced  int64   `json:"bytes_reduced"`
	SavingsPct    float64 `json:"savings_pct"`
	Unit          string  `json:"unit"`
	TokenState    string  `json:"token_state"`
	State         string  `json:"state"`
}

const compactionLogName = "context-compactions.jsonl"

// CompactionRecord is one append-only tool-output observation, grouped by harness.
// It never stores tool output, prompts, file paths, or a raw session id.
type CompactionRecord struct {
	Timestamp    string `json:"timestamp"`
	Harness      string `json:"harness"`
	SessionHash  string `json:"session_hash,omitempty"`
	Tool         string `json:"tool,omitempty"`
	OutputBytes  int64  `json:"output_bytes"`
	ReducedBytes int64  `json:"reduced_bytes"`
	Applied      bool   `json:"applied"`
	Error        bool   `json:"error"`
}

func newCompactionRecord(sessionHash, harness, toolName string, output []byte, isError bool, res compressor.Result, now time.Time) CompactionRecord {
	rec := CompactionRecord{
		Timestamp:   now.UTC().Format(time.RFC3339Nano),
		Harness:     harness,
		OutputBytes: int64(len(output)),
		Applied:     res.Applied,
		Error:       isError,
	}
	if sessionDigest(sessionHash) {
		rec.SessionHash = sessionHash
	}
	if validToolName(toolName) {
		rec.Tool = toolName
	}
	if res.Applied {
		rec.ReducedBytes = int64(res.OriginalBytes - res.ReducedBytes)
	}
	return rec
}

func validToolName(name string) bool {
	if len(name) < 1 || len(name) > 40 {
		return false
	}
	for _, c := range name {
		switch {
		case c >= 'A' && c <= 'Z':
		case c >= 'a' && c <= 'z':
		case c >= '0' && c <= '9':
		case c == '_' || c == '-':
		default:
			return false
		}
	}
	return true
}

func (s *Store) appendCompaction(rec CompactionRecord) error {
	path := s.compactionPath
	if path == "" {
		var err error
		path, err = paths.Join(compactionLogName)
		if err != nil {
			return err
		}
	}
	return AppendCompaction(path, rec)
}

// AppendCompaction appends one JSON line to path. The line is the record only.
// Writers in other processes share path.lock. Records older than 90 days, and
// records past the newest 5000, are dropped on the same locked rewrite.
func AppendCompaction(path string, rec CompactionRecord) error {
	return withPathLock(path, func() error {
		return appendCompactionUnlocked(path, rec)
	})
}

func appendCompactionUnlocked(path string, rec CompactionRecord) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	line, err := json.Marshal(rec)
	if err != nil {
		_ = f.Close()
		return err
	}
	if _, err := f.Write(append(line, '\n')); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return retainCompaction(path, time.Now().UTC())
}

func retainCompaction(path string, now time.Time) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	lines := bytes.Split(raw, []byte("\n"))
	kept := make([][]byte, 0, len(lines))
	for _, line := range lines {
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		var rec CompactionRecord
		if err := json.Unmarshal(line, &rec); err != nil {
			kept = append(kept, line)
			continue
		}
		ts, err := time.Parse(time.RFC3339Nano, rec.Timestamp)
		if err == nil && now.Sub(ts) > compactionMaxAge {
			continue
		}
		kept = append(kept, line)
	}
	if len(kept) > compactionMaxRecords {
		kept = kept[len(kept)-compactionMaxRecords:]
	}
	nonEmpty := 0
	for _, line := range lines {
		if len(bytes.TrimSpace(line)) > 0 {
			nonEmpty++
		}
	}
	if len(kept) == nonEmpty {
		return nil
	}
	payload := append(bytes.Join(kept, []byte("\n")), '\n')
	tmp := fmt.Sprintf("%s.%d.%d.tmp", path, os.Getpid(), now.UnixNano())
	if err := os.WriteFile(tmp, payload, 0600); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

// SummarizeCompactionLog aggregates the append-only log by harness.
// Any line outside the record contract rejects the whole file.
func SummarizeCompactionLog(path string) (map[string]HarnessCompaction, error) {
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return map[string]HarnessCompaction{}, nil
		}
		return nil, err
	}
	var out map[string]HarnessCompaction
	err := withPathLock(path, func() error {
		var sumErr error
		out, sumErr = summarizeCompactionLogUnlocked(path)
		return sumErr
	})
	return out, err
}

func summarizeCompactionLogUnlocked(path string) (map[string]HarnessCompaction, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]HarnessCompaction{}, nil
		}
		return nil, err
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	acc := map[string]*HarnessCompaction{}
	for dec.More() {
		var rec CompactionRecord
		if err := dec.Decode(&rec); err != nil {
			return nil, fmt.Errorf("compaction log rejected")
		}
		if !validToolName(rec.Harness) {
			return nil, fmt.Errorf("compaction log rejected")
		}
		if rec.OutputBytes < 0 || rec.ReducedBytes < 0 || rec.ReducedBytes > rec.OutputBytes {
			return nil, fmt.Errorf("compaction log rejected")
		}
		row := acc[rec.Harness]
		if row == nil {
			row = &HarnessCompaction{
				Harness:    rec.Harness,
				Unit:       "bytes",
				TokenState: string(StateUnavailable),
				State:      string(StateObserved),
			}
			acc[rec.Harness] = row
		}
		row.TotalCommands++
		row.BytesBefore += rec.OutputBytes
		reduced := rec.ReducedBytes
		if rec.Error {
			reduced = 0
		}
		row.BytesReduced += reduced
	}
	out := make(map[string]HarnessCompaction, len(acc))
	for name, row := range acc {
		if row.BytesReduced > row.BytesBefore {
			row.BytesReduced = row.BytesBefore
		}
		row.BytesAfter = row.BytesBefore - row.BytesReduced
		if row.BytesBefore > 0 {
			row.SavingsPct = float64(row.BytesReduced) / float64(row.BytesBefore) * 100
		}
		out[name] = *row
	}
	return out, nil
}

// SummarizeByHarness reads this store's compaction log. A line outside the
// record contract returns an error and no harness rows.
func (s *Store) SummarizeByHarness() (map[string]HarnessCompaction, error) {
	if s == nil || s.compactionPath == "" {
		return map[string]HarnessCompaction{}, nil
	}
	return SummarizeCompactionLog(s.compactionPath)
}

func withPathLock(path string, fn func() error) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	f, err := os.OpenFile(path+".lock", os.O_CREATE|os.O_RDWR, 0600)
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
