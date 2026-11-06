// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

// Package mainwork measures the mechanical work the main session does itself:
// reads, searches and single commands that a small-tier subagent could take.
// A record holds the tool name, a category code, the harness and the hashed
// session id; the session model's tier is resolved at report time from the
// session's latest decision (summary.go), never per call. Never a command, path, pattern or prompt (same rule as
// internal/execwrap/skips.go). Nothing here routes or rewrites.
package mainwork

import (
	"bufio"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/tiagovilasboas/downshift/internal/core"
	"github.com/tiagovilasboas/downshift/internal/paths"
	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

// FileName is the log of main-session tool calls.
const FileName = "main-work.jsonl"

// Record is one main-session tool call.
type Record struct {
	TS       string `json:"ts"`
	Harness  string `json:"harness"`
	Tool     string `json:"tool"`
	Category string `json:"category"`
	// Session is the hashed session id; the report maps it to the session
	// model's tier. Tier is only read from older records that stored it.
	Session string `json:"session,omitempty"`
	Tier    string `json:"tier,omitempty"`
	// ResultBytesBucket is the size class of a read result (SizeBucket); absent
	// when the harness did not expose the size or the tool is not a read.
	ResultBytesBucket string `json:"result_bytes_bucket,omitempty"`
	// ResultBytes is the exact byte length behind the bucket (a number, never content).
	ResultBytes int `json:"result_bytes,omitempty"`
}

// Call is what an adapter parsed from a tool-event payload. Subagent is true
// when the payload says the call came from a subagent (an agent id is present).
type Call struct {
	Harness  string
	Tool     string
	Session  string // hashed session id, may be empty
	Subagent bool
	// ResultBytes is the byte length of the tool's result as the adapter
	// measured it; 0 means unknown. Only the length crosses, never the text.
	ResultBytes int
}

// Path is main-work.jsonl in the state directory.
func Path() (string, error) { return paths.Join(FileName) }

// Build turns a call into a record. ok is false when nothing should be
// recorded: a subagent call, an unnamed tool or a harness that does not
// observe main-session tools.
func Build(c Call, caps core.HarnessCapabilities, now time.Time) (Record, bool) {
	if c.Subagent || c.Tool == "" || !caps.MainWorkObserved() {
		return Record{}, false
	}
	cat := caps.CategoryOf(c.Tool)
	rec := Record{
		TS: now.UTC().Format(time.RFC3339), Harness: c.Harness, Tool: c.Tool,
		Category: string(cat), Session: c.Session,
	}
	if cat == core.CategoryRead {
		rec.ResultBytesBucket = SizeBucket(c.ResultBytes)
		if rec.ResultBytesBucket != "" {
			rec.ResultBytes = c.ResultBytes
		}
	}
	return rec, true
}

// Append writes one record, rotating the file like the other logs. Callers
// ignore the error.
func Append(p string, rec Record) error {
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return err
	}
	_, _ = telemetry.RotationPolicyFromEnv().MaybeRotate(p)
	line, err := json.Marshal(rec)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(p, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(append(line, '\n'))
	return err
}

// Observe records a call in the default log. Fail-open: errors are dropped.
func Observe(c Call, caps core.HarnessCapabilities) {
	rec, ok := Build(c, caps, time.Now())
	if !ok {
		return
	}
	if p, err := Path(); err == nil {
		_ = Append(p, rec)
	}
}

// Read returns the records at p; a missing file is empty.
func Read(p string) ([]Record, error) {
	f, err := os.Open(p)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()
	var out []Record
	sc := bufio.NewScanner(f)
	sc.Buffer(nil, telemetry.MaxLineBytes)
	for sc.Scan() {
		var rec Record
		if json.Unmarshal(sc.Bytes(), &rec) == nil && rec.Harness != "" {
			out = append(out, rec)
		}
	}
	return out, sc.Err()
}
