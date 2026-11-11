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
//
// Session is the session reference (SessionRef) the kept-output hint handed
// the agent, so a read is charged to the session that kept the output. It is
// empty for a read that came without one (the MCP tool, older rows) and then
// omitted from the row.
type Retrieval struct {
	TS      string `json:"ts"`
	Kept    string `json:"kept"`
	Bytes   int64  `json:"bytes"`
	Session string `json:"session,omitempty"`
}

// sessionRefLen is the length of a session reference: the first 12 hex
// characters of the hashed session, like a CCR id.
const sessionRefLen = 12

// SessionRef returns the short stable reference of a hashed session: its first
// 12 characters, when h is 12 to 64 lowercase hex characters. Anything else
// gives "" (no session). The kept-output hint prints it, `retrieve` records
// it, and matching compares it with the prefix of an event's Session.
func SessionRef(h string) string {
	if len(h) < sessionRefLen || len(h) > 64 {
		return ""
	}
	for _, c := range h {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return ""
		}
	}
	return h[:sessionRefLen]
}

// RecordRetrieval appends one retrieval. The CLI and the MCP tool both call it
// with the length of what they returned. session is a hashed session or its
// reference (SessionRef), "" when the read came without one; anything else is
// stored as no session. It never fails the read: errors are dropped, and an id
// that is not 12 lowercase hex characters or a read of zero bytes records
// nothing.
func RecordRetrieval(id, session string, returned int, now time.Time) {
	if returned <= 0 || !validKeptID(id) {
		return
	}
	path, err := paths.Join(RetrievalsFileName)
	if err != nil {
		return
	}
	line, err := json.Marshal(Retrieval{TS: now.UTC().Format(time.RFC3339), Kept: id, Bytes: int64(returned), Session: SessionRef(session)})
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
// beside the exec log at eventsPath (rotated generation included), for the
// reads that carry no session. A missing file is an empty map; malformed lines
// are skipped.
func ReadRetrievals(eventsPath string) map[string]int64 {
	byID, _ := readRetrievalLog(eventsPath)
	return byID
}

// readRetrievalLog splits the log into reads without a session (per id) and
// reads of one session (per session reference and id, see scopedKey).
func readRetrievalLog(eventsPath string) (byID, scoped map[string]int64) {
	byID, scoped = map[string]int64{}, map[string]int64{}
	_ = scanLog(retrievalsPath(eventsPath), func(line []byte) {
		var r Retrieval
		if json.Unmarshal(line, &r) != nil || r.Bytes <= 0 || !validKeptID(r.Kept) {
			return
		}
		if ref := SessionRef(r.Session); ref != "" {
			scoped[scopedKey(ref, r.Kept)] += r.Bytes
			return
		}
		byID[r.Kept] += r.Bytes
	})
	return byID, scoped
}

// scopedKey joins a session reference and a CCR id.
func scopedKey(ref, id string) string { return ref + "/" + id }

// applyLogged sets RetrievedBytes on evs from the retrievals log beside
// eventsPath: first the reads of one session against the runs of that session,
// then the reads without a session against any run that kept the id.
func applyLogged(evs []Event, eventsPath string) []Event {
	byID, scoped := readRetrievalLog(eventsPath)
	evs = applyKeyed(evs, scoped, func(ev Event) string {
		if ref := SessionRef(ev.Session); ref != "" && ev.Kept != "" {
			return scopedKey(ref, ev.Kept)
		}
		return ""
	})
	return ApplyRetrievals(evs, byID)
}

// ApplyRetrievals matches the bytes read back per id (reads without a session)
// to the runs that kept that id and sets their RetrievedBytes. Content
// addressing gives two runs with the same output the same id; the bytes go to
// the runs in log order, each up to what it saved, and any remainder to the
// last of them, so the total shown is what was read back. A run whose id was
// never read back, and an id no run in evs kept, change nothing. evs is
// modified in place and returned.
func ApplyRetrievals(evs []Event, byID map[string]int64) []Event {
	return applyKeyed(evs, byID, func(ev Event) string { return ev.Kept })
}

// applyKeyed is the one arithmetic of ApplyRetrievals over any grouping of
// runs: keyOf names the group of a run ("" for none) and totals holds the
// bytes read back per group. Bytes already on a run (RetrievedBytes) count
// against what it can still take.
func applyKeyed(evs []Event, totals map[string]int64, keyOf func(Event) string) []Event {
	if len(totals) == 0 {
		return evs
	}
	last := map[string]int{}
	for i, ev := range evs {
		if k := keyOf(ev); k != "" && totals[k] > 0 {
			last[k] = i
		}
	}
	left := map[string]int64{}
	for k, n := range totals {
		left[k] = n
	}
	for i := range evs {
		k := keyOf(evs[i])
		if k == "" || left[k] <= 0 {
			continue
		}
		take := evs[i].RawBytes - evs[i].EmittedBytes - evs[i].RetrievedBytes
		if take < 0 {
			take = 0
		}
		if take > left[k] || i == last[k] {
			take = left[k]
		}
		evs[i].RetrievedBytes += take
		left[k] -= take
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
