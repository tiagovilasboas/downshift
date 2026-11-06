// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package execwrap

import (
	"encoding/json"
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
	// PassthroughNonZeroExit: the program failed (or never started) and the
	// failed output stayed raw. Failing go test, vitest, playwright, jest,
	// pytest and cargo test runs are compacted instead when every failure
	// line is kept.
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
	// Folded marks a compacted run whose output was a lossy fold with the raw
	// kept in the CCR store, apart from the lossless compressor. Only true
	// rows carry it; Compacted is true on those rows too.
	Folded bool `json:"folded,omitempty"`
	// Passthrough is one of the Passthrough* reasons when Compacted is false,
	// and empty when Compacted is true. Empty on a not-compacted row also means
	// the reason was not recorded (output streamed before the gate decided).
	Passthrough string `json:"passthrough,omitempty"`
	// PassthroughDetail refines Passthrough "unrecognized_output" with one of
	// the Detail* codes (danger_word, ansi, cr, ...). Optional: rows written
	// before this field, and every other passthrough reason, carry none.
	PassthroughDetail string `json:"passthrough_detail,omitempty"`
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
	// Model is the LLM model that ran this command, when detectable from the
	// harness's session data (e.g. KiroCrew's session files). Empty when the
	// harness does not expose model information or when detection fails.
	Model string `json:"model,omitempty"`
	// Client names the harness client that produced this command, when the
	// harness exposes it to the child shell: for Claude Code,
	// CLAUDE_CODE_ENTRYPOINT (values such as claude-desktop, cli, sdk-cli,
	// sdk-ts). The caller fills it from the harness environment through
	// telemetry.ClientOrDrop, so a compacted command can be tied to its client
	// instead of being matched by time. Empty when the harness exposes no
	// client or the value is dropped by the sanitizer; absent on rows written
	// before this field was added.
	Client string `json:"client,omitempty"`
	// KeptID is the CCR id (12 lowercase hex) the run's raw output was stored
	// under, when it was. It is what a retrieval is matched against. A hash of
	// the redacted output: never the command or the output. Empty when nothing
	// was stored, and on rows written before this field.
	KeptID string `json:"kept_id,omitempty"`
	// RetrievedBytes is filled by the readers, never written: the bytes fetched
	// back with `retrieve` for KeptID that were charged to this run (see
	// attachRetrievals). NetSaved subtracts them from the saving.
	RetrievedBytes int64 `json:"-"`
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
		Folded:       r.Folded,
		KeptID:       r.KeptID,
		Passthrough:  r.Passthrough,

		PassthroughDetail: r.PassthroughDetail,
	}
}

// EventsPath is exec-events.jsonl in the state directory.
func EventsPath() (string, error) { return paths.Join(EventsFileName) }

// Append writes one record. Callers ignore the error: telemetry never changes
// what the wrapped command prints or returns.
func Append(path string, ev Event) error {
	line, err := json.Marshal(ev)
	if err != nil {
		return err
	}
	return appendLine(path, line)
}

// ReadEvents reads every record from the exec log at path, with the bytes
// fetched back by `retrieve` (the retrievals log beside it) attached to the
// runs they belong to. A missing file is an empty slice, not an error.
// Malformed lines are skipped, mirroring Summarize. Callers that only need
// totals should use Summarize instead.
func ReadEvents(path string) ([]Event, error) {
	return readEvents(path, "")
}

// readEvents is ReadEvents for one hashed session (all of them when session is
// empty). The retrievals are charged to the runs of the whole log first, so a
// session sees only what was fetched back for its own runs.
func readEvents(path, session string) ([]Event, error) {
	var out []Event
	err := scanLog(path, func(line []byte) {
		var ev Event
		if json.Unmarshal(line, &ev) == nil {
			out = append(out, ev)
		}
	})
	if err != nil {
		return out, err
	}
	rets, rerr := ReadRetrievals(retrievalsBeside(path))
	attachRetrievals(out, rets)
	if session != "" {
		kept := out[:0]
		for _, ev := range out {
			if ev.Session == session {
				kept = append(kept, ev)
			}
		}
		out = kept
	}
	return out, rerr
}
