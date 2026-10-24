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

// Passthrough reasons: why an exec run's output went through raw. Each is a
// constant, so the log holds no free text.
const (
	// PassthroughSmallOutput: the output was empty, or the compressed summary
	// would not be shorter than the raw output.
	PassthroughSmallOutput = "small_output"
	// PassthroughNonZeroExit: the program failed (or never started), and a
	// failed run passes through by design.
	PassthroughNonZeroExit = "nonzero_exit"
	// PassthroughUnrecognized: the family's compressor did not recognise the
	// output shape, or the family has no compressor.
	PassthroughUnrecognized = "unrecognized_output"
	// PassthroughRefused: the command was not allowlisted and did not run
	// (exit 126).
	PassthroughRefused = "refused"
)

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
	// Passthrough is one of the Passthrough* reasons when Compacted is false,
	// and empty when Compacted is true. Empty on a not-compacted row also means
	// the reason was not recorded (output streamed before the gate decided).
	Passthrough string `json:"passthrough,omitempty"`
	// Session is the hashed session id (telemetry.HashSessionID) the hook put on
	// the wrapped command; empty when the harness gave none. Never the raw id.
	Session string `json:"session,omitempty"`
	// Harness is the harness that wrapped this call ("claude-code", "kirocrew",
	// etc.) when one can be identified from the adapter's environment, or
	// "unknown" when none is detected. Always present in new rows.
	Harness string `json:"harness,omitempty"`
	// Entry is "hook" when the call arrived through a harness hook
	// (--session= was set by the adapter), "direct" when the agent or user ran
	// downshift compactor directly, or "unknown" for rows written before this
	// field was added (old readers can ignore it; new readers treat absence as
	// "unknown").
	Entry string `json:"entry,omitempty"`
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
		Passthrough:  r.Passthrough,
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
	Failures             int64            `json:"failures,omitempty"`
	RawBytes             int64            `json:"raw_bytes"`
	EmittedBytes         int64            `json:"emitted_bytes"`
	SavedBytes           int64            `json:"saved_bytes"`
	EstimatedTokensSaved int64            `json:"estimated_tokens_saved"`
	TokenEstimate        string           `json:"token_estimate"`
	ByFamily             map[Family]int64 `json:"commands_by_family,omitempty"`
	SavedBytesByFamily   map[Family]int64 `json:"saved_bytes_by_family,omitempty"`
	CompactedByFamily    map[Family]int64 `json:"compacted_by_family,omitempty"`
	FailuresByFamily     map[Family]int64 `json:"failures_by_family,omitempty"`
}

// Summarize reads the exec log at path. A missing file is an empty summary.
// Malformed lines are skipped.
func Summarize(path string) (Summary, error) { return SummarizeSession(path, "") }

// ReadEvents reads every record from the exec log at path. A missing file is
// an empty slice, not an error. Malformed lines are skipped, mirroring
// Summarize. Callers that only need totals should use Summarize instead.
func ReadEvents(path string) ([]Event, error) {
	f, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()
	var out []Event
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		var ev Event
		if json.Unmarshal(sc.Bytes(), &ev) != nil {
			continue
		}
		out = append(out, ev)
	}
	return out, sc.Err()
}

// SummarizeSession totals only the records of one hashed session; an empty
// session totals the whole log. Records written before the session field
// existed belong to no session.
func SummarizeSession(path, session string) (Summary, error) {
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
		if session != "" && ev.Session != session {
			continue
		}
		s.Commands++
		if ev.Compacted {
			s.Compacted++
			if s.CompactedByFamily == nil {
				s.CompactedByFamily = map[Family]int64{}
			}
			s.CompactedByFamily[ev.Family]++
		}
		if ev.ExitClass != "" && ev.ExitClass != ExitClassZero {
			s.Failures++
			if s.FailuresByFamily == nil {
				s.FailuresByFamily = map[Family]int64{}
			}
			s.FailuresByFamily[ev.Family]++
		}
		s.RawBytes += ev.RawBytes
		s.EmittedBytes += ev.EmittedBytes
		if ev.RawBytes > ev.EmittedBytes {
			saved := ev.RawBytes - ev.EmittedBytes
			s.SavedBytes += saved
			if s.SavedBytesByFamily == nil {
				s.SavedBytesByFamily = map[Family]int64{}
			}
			s.SavedBytesByFamily[ev.Family] += saved
		}
		if s.ByFamily == nil {
			s.ByFamily = map[Family]int64{}
		}
		s.ByFamily[ev.Family]++
	}
	s.EstimatedTokensSaved = s.SavedBytes / BytesPerToken
	return s, sc.Err()
}
