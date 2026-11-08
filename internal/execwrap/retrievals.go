// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package execwrap

import (
	"encoding/json"
	"path/filepath"
	"time"

	"github.com/tiagovilasboas/downshift-ai/internal/paths"
)

// RetrievalsFileName is the log of bytes `downshift retrieve` handed back to the
// agent, under the state directory. A compacted run that kept its raw output
// saved bytes only until the agent read them back; this log is how the
// summaries find out.
const RetrievalsFileName = "exec-retrievals.jsonl"

// Retrieval is one read of kept output. It carries the CCR id (a hash of the
// stored text), the number of bytes returned and the time. Never the output,
// the query, a path or a command.
type Retrieval struct {
	TS    string `json:"ts"`
	Kept  string `json:"kept"`
	Bytes int64  `json:"bytes"`
}

// RecordRetrieval appends one retrieval. The CLI and the MCP tool both call it
// with the length of what they returned. It never fails the read: errors are
// dropped, and an id that is not 12 lowercase hex characters or a read of zero
// bytes records nothing.
func RecordRetrieval(id string, returned int, now time.Time) {
	if returned <= 0 || !validKeptID(id) {
		return
	}
	path, err := paths.Join(RetrievalsFileName)
	if err != nil {
		return
	}
	line, err := json.Marshal(Retrieval{TS: now.UTC().Format(time.RFC3339), Kept: id, Bytes: int64(returned)})
	if err != nil {
		return
	}
	_ = appendLine(path, line)
}

// retrievalsPath is the retrievals log that sits beside the exec log at
// eventsPath, so a test or a caller with its own events file reads its own.
func retrievalsPath(eventsPath string) string {
	return filepath.Join(filepath.Dir(eventsPath), RetrievalsFileName)
}

// ReadRetrievals totals the bytes read back per CCR id from the retrievals log
// beside the exec log at eventsPath (rotated generation included). A missing
// file is an empty map; malformed lines are skipped.
func ReadRetrievals(eventsPath string) map[string]int64 {
	totals := map[string]int64{}
	_ = scanLog(retrievalsPath(eventsPath), func(line []byte) {
		var r Retrieval
		if json.Unmarshal(line, &r) != nil || r.Bytes <= 0 || !validKeptID(r.Kept) {
			return
		}
		totals[r.Kept] += r.Bytes
	})
	return totals
}

// ApplyRetrievals matches the bytes read back per id to the runs that kept that
// id and sets their RetrievedBytes. Content addressing gives two runs with the
// same output the same id; the bytes go to the runs in log order, each up to
// what it saved, and any remainder to the last of them, so the total shown is
// what was read back. A run whose id was never read back, and an id no run in
// evs kept, change nothing. evs is modified in place and returned.
func ApplyRetrievals(evs []Event, byID map[string]int64) []Event {
	if len(byID) == 0 {
		return evs
	}
	last := map[string]int{}
	for i, ev := range evs {
		if ev.Kept != "" && byID[ev.Kept] > 0 {
			last[ev.Kept] = i
		}
	}
	left := map[string]int64{}
	for id, n := range byID {
		left[id] = n
	}
	for i := range evs {
		id := evs[i].Kept
		if id == "" || left[id] <= 0 {
			continue
		}
		take := evs[i].RawBytes - evs[i].EmittedBytes
		if take < 0 {
			take = 0
		}
		if take > left[id] || i == last[id] {
			take = left[id]
		}
		evs[i].RetrievedBytes += take
		left[id] -= take
	}
	return evs
}

// validKeptID reports whether id looks like a CCR id: 12 lowercase hex
// characters. execwrap does not import internal/ccr, so it checks the shape.
func validKeptID(id string) bool {
	if len(id) != 12 {
		return false
	}
	for _, c := range id {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}
