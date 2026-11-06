// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package execwrap

import (
	"encoding/json"
	"path/filepath"
	"time"

	"github.com/tiagovilasboas/downshift/internal/ccr"
	"github.com/tiagovilasboas/downshift/internal/paths"
)

// RetrievalsFileName is the log of bytes fetched back with `retrieve`. It sits
// next to the exec log (same directory, same rotation) and is read together
// with it. It is a separate file so a log written before it existed, and a
// reader that does not know it, see exactly what they saw before.
const RetrievalsFileName = "exec-retrievals.jsonl"

// Retrieval is one `retrieve` that returned bytes to the model. It carries the
// 12-hex id, the byte count and the time: never the command, the output, the
// query (grep, lines), a path or a prompt.
type Retrieval struct {
	TS    string `json:"ts"`
	ID    string `json:"id"`
	Bytes int64  `json:"bytes"`
}

// RetrievalsPath is exec-retrievals.jsonl in the state directory.
func RetrievalsPath() (string, error) { return paths.Join(RetrievalsFileName) }

// retrievalsBeside is the retrievals log that goes with the exec log at
// eventsPath: the same directory.
func retrievalsBeside(eventsPath string) string {
	return filepath.Join(filepath.Dir(eventsPath), RetrievalsFileName)
}

// AppendRetrieval writes one record, rotating like the exec log.
func AppendRetrieval(path string, r Retrieval) error {
	line, err := json.Marshal(r)
	if err != nil {
		return err
	}
	return appendLine(path, line)
}

// ReadRetrievals reads the retrievals log at path, rotated generation first.
// A missing file is empty; malformed lines, ids of the wrong shape and
// non-positive counts are skipped.
func ReadRetrievals(path string) ([]Retrieval, error) {
	var out []Retrieval
	err := scanLog(path, func(line []byte) {
		var r Retrieval
		if json.Unmarshal(line, &r) == nil && ccr.ValidID(r.ID) && r.Bytes > 0 {
			out = append(out, r)
		}
	})
	return out, err
}

// RecordRetrieval logs that n bytes of the output kept under id reached the
// model. Best effort: a log failure never changes what retrieve returns.
func RecordRetrieval(id string, n int64, now time.Time) {
	if n <= 0 || !ccr.ValidID(id) {
		return
	}
	if path, err := RetrievalsPath(); err == nil {
		_ = AppendRetrieval(path, Retrieval{TS: now.UTC().Format(time.RFC3339), ID: id, Bytes: n})
	}
}

// QueryError is a Retrieve failure caused by the query (a bad grep pattern),
// not by the store, so a caller can answer it as a usage error.
type QueryError struct{ Err error }

func (e *QueryError) Error() string { return e.Err.Error() }
func (e *QueryError) Unwrap() error { return e.Err }

// Retrieve is the one retrieval path, used by `downshift retrieve` and the MCP
// tool `retrieve`. It reads the kept output for id from store, applies q, cuts
// the result to maxBytes when maxBytes is positive (truncated says it did) and
// records the bytes returned. The errors are the ccr ones, or a *QueryError, so
// each caller words its own message. A retrieval that fails, or returns nothing, records nothing.
func Retrieve(store ccr.Store, id string, q ccr.Query, maxBytes int) (out []byte, truncated bool, err error) {
	data, err := store.Get(id)
	if err != nil {
		return nil, false, err
	}
	out, err = ccr.Slice(data, q)
	if err != nil {
		return nil, false, &QueryError{Err: err}
	}
	if maxBytes > 0 && len(out) > maxBytes {
		out, truncated = out[:maxBytes:maxBytes], true
	}
	RecordRetrieval(id, int64(len(out)), time.Now())
	return out, truncated, nil
}
