// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package execwrap

import (
	"bufio"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/tiagovilasboas/downshift/internal/paths"
)

// EventsFileName is the prompt-free exec log under the state directory.
const EventsFileName = "exec-events.jsonl"

// BytesPerToken is the rough ratio behind EstimatedTokensSaved. It is an
// estimate, labelled as one wherever it is shown.
const BytesPerToken = 4

// Event is one `downshift exec` run. It carries the family, byte counts, the
// exit-code class and whether the output was compacted: never the command
// text, arguments, paths, working directory or output.
type Event struct {
	TS           string `json:"ts"`
	Family       Family `json:"family"`
	RawBytes     int64  `json:"raw_bytes"`
	EmittedBytes int64  `json:"emitted_bytes"`
	ExitClass    string `json:"exit_class"`
	Compacted    bool   `json:"compacted"`
}

// EventFrom builds the log record for a Result.
func EventFrom(r Result, now time.Time) Event {
	family := r.Family
	if family == FamilyNone {
		family = "other"
	}
	return Event{
		TS:           now.UTC().Format(time.RFC3339),
		Family:       family,
		RawBytes:     r.RawBytes,
		EmittedBytes: r.EmittedBytes,
		ExitClass:    r.ExitClass,
		Compacted:    r.Compacted,
	}
}

// EventsPath is exec-events.jsonl in the state directory.
func EventsPath() (string, error) { return paths.Join(EventsFileName) }

// Append writes one record. Callers ignore the error: telemetry never changes
// what the wrapped command prints or returns.
func Append(path string, ev Event) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	line, err := json.Marshal(ev)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(append(line, '\n')) // one short line: an atomic O_APPEND write
	return err
}

// Summary totals the exec log. SavedBytes is measured; EstimatedTokensSaved
// is SavedBytes / BytesPerToken and is an estimate.
type Summary struct {
	Commands             int64            `json:"commands"`
	Compacted            int64            `json:"compacted"`
	RawBytes             int64            `json:"raw_bytes"`
	EmittedBytes         int64            `json:"emitted_bytes"`
	SavedBytes           int64            `json:"saved_bytes"`
	EstimatedTokensSaved int64            `json:"estimated_tokens_saved"`
	TokenEstimate        string           `json:"token_estimate"`
	ByFamily             map[Family]int64 `json:"commands_by_family,omitempty"`
}

// Summarize reads the exec log at path. A missing file is an empty summary.
// Malformed lines are skipped.
func Summarize(path string) (Summary, error) {
	s := Summary{TokenEstimate: "estimated: saved_bytes / 4"}
	f, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return s, nil
		}
		return s, err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		var ev Event
		if json.Unmarshal(sc.Bytes(), &ev) != nil {
			continue
		}
		s.Commands++
		if ev.Compacted {
			s.Compacted++
		}
		s.RawBytes += ev.RawBytes
		s.EmittedBytes += ev.EmittedBytes
		if ev.RawBytes > ev.EmittedBytes {
			s.SavedBytes += ev.RawBytes - ev.EmittedBytes
		}
		if s.ByFamily == nil {
			s.ByFamily = map[Family]int64{}
		}
		s.ByFamily[ev.Family]++
	}
	s.EstimatedTokensSaved = s.SavedBytes / BytesPerToken
	return s, sc.Err()
}
