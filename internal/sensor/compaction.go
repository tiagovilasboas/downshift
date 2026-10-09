// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package sensor

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/tiagovilasboas/downshift/internal/compressor"
	"github.com/tiagovilasboas/downshift/internal/paths"
)

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
func AppendCompaction(path string, rec CompactionRecord) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer f.Close()

	line, err := json.Marshal(rec)
	if err != nil {
		return err
	}
	if _, err := f.Write(append(line, '\n')); err != nil {
		return err
	}
	return f.Sync()
}
