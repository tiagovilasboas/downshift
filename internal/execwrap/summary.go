// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package execwrap

import (
	"encoding/json"
)

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
	// PassthroughByReason counts the runs that were not compacted, by the
	// Passthrough reason on the row (PassthroughNotRecorded when the row has
	// none). Its values add up to Commands - Compacted.
	PassthroughByReason map[string]int64 `json:"passthrough_by_reason,omitempty"`
	// PassthroughByDetail counts the not-compacted rows that carry a
	// passthrough_detail code; older rows and other reasons add nothing.
	PassthroughByDetail map[string]int64 `json:"passthrough_by_detail,omitempty"`
	// PassthroughRawBytesByFamily is the raw bytes of the runs that were not
	// compacted, by family: the output that reached the model whole.
	PassthroughRawBytesByFamily map[Family]int64 `json:"passthrough_raw_bytes_by_family,omitempty"`
	// SkipsByReason counts the Bash calls the hook declined to wrap, by Skip*
	// reason, from the skip log (filled by SummarizeSkips; no command text).
	SkipsByReason map[string]int64 `json:"skips_by_reason,omitempty"`
	// SkipsByMetachar counts the metachar skips by the class of the first
	// shell metacharacter (semicolon, pipe, ...); filled by SummarizeSkipClasses.
	SkipsByMetachar map[string]int64 `json:"skips_by_metachar,omitempty"`
	// SkipsByKind counts the metachar skips by MetacharKind (heredoc, subst,
	// redirect, pipe_grep, ...); filled by SummarizeSkipKinds.
	SkipsByKind map[string]int64 `json:"skips_by_kind,omitempty"`
	// FailuresCompacted and FailuresPassedRaw split Failures by whether the
	// failing run was compacted or passed through raw; they add up to Failures.
	FailuresCompacted int64 `json:"failures_compacted,omitempty"`
	FailuresPassedRaw int64 `json:"failures_passed_raw,omitempty"`
	// FoldedRuns and FoldedSavedBytes are the part of Compacted and SavedBytes
	// that came from a lossy fold (raw kept in CCR), not the lossless
	// compressor. Totals above still include them.
	FoldedRuns       int64 `json:"folded_runs,omitempty"`
	FoldedSavedBytes int64 `json:"folded_saved_bytes,omitempty"`
}

// PassthroughNotRecorded is the PassthroughByReason key of a not-compacted row
// that carries no reason (the output streamed before the gate decided).
const PassthroughNotRecorded = "not_recorded"

// addPassthrough counts a run that was not compacted: its reason and its raw
// bytes by family.
func (s *Summary) addPassthrough(ev Event) {
	reason := ev.Passthrough
	if reason == "" {
		reason = PassthroughNotRecorded
	}
	if s.PassthroughByReason == nil {
		s.PassthroughByReason = map[string]int64{}
		s.PassthroughRawBytesByFamily = map[Family]int64{}
	}
	s.PassthroughByReason[reason]++
	if ev.PassthroughDetail != "" {
		if s.PassthroughByDetail == nil {
			s.PassthroughByDetail = map[string]int64{}
		}
		s.PassthroughByDetail[ev.PassthroughDetail]++
	}
	s.PassthroughRawBytesByFamily[ev.Family] += ev.RawBytes
}

// addFailure counts a failing run, apart by family and by whether it was
// compacted or passed raw.
func (s *Summary) addFailure(ev Event) {
	s.Failures++
	if s.FailuresByFamily == nil {
		s.FailuresByFamily = map[Family]int64{}
	}
	s.FailuresByFamily[ev.Family]++
	if ev.Compacted {
		s.FailuresCompacted++
	} else {
		s.FailuresPassedRaw++
	}
}

// Summarize reads the exec log at path. A missing file is an empty summary.
// Malformed lines are skipped.
func Summarize(path string) (Summary, error) { return SummarizeSession(path, "") }

// SummarizeSession totals only the records of one hashed session; an empty
// session totals the whole log. Records written before the session field
// existed belong to no session.
func SummarizeSession(path, session string) (Summary, error) {
	var evs []Event
	err := scanLog(path, func(line []byte) {
		var ev Event
		if json.Unmarshal(line, &ev) != nil {
			return
		}
		if session != "" && ev.Session != session {
			return
		}
		evs = append(evs, ev)
	})
	return SummarizeEvents(evs), err
}

// SummarizeEvents totals already-read records. It is the one place the
// savings arithmetic lives: the file summary and the accounting package both
// call it.
func SummarizeEvents(evs []Event) Summary {
	s := Summary{TokenEstimate: "estimated: saved_bytes / 4"}
	for _, ev := range evs {
		s.Commands++
		if ev.Compacted {
			s.Compacted++
			if s.CompactedByFamily == nil {
				s.CompactedByFamily = map[Family]int64{}
			}
			s.CompactedByFamily[ev.Family]++
		} else {
			s.addPassthrough(ev)
		}
		if ev.ExitClass != "" && ev.ExitClass != ExitClassZero {
			s.addFailure(ev)
		}
		s.RawBytes += ev.RawBytes
		s.EmittedBytes += ev.EmittedBytes
		if ev.Folded {
			s.FoldedRuns++
		}
		if ev.RawBytes > ev.EmittedBytes {
			saved := ev.RawBytes - ev.EmittedBytes
			s.SavedBytes += saved
			if ev.Folded {
				s.FoldedSavedBytes += saved
			}
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
	return s
}
