// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package hookport

import "time"

// ToolResultSample is one kind of tool result the model received, as a
// harness's transcript records it: a CCR id the result's retrieve hint names,
// the UTF-8 length of the result's text, and how many distinct results had that
// pair. Never the text itself.
type ToolResultSample struct {
	ID    string
	Bytes int
	// Count is how many distinct results had this id and length; 0 means 1.
	// Each one can honor at most one row.
	Count int
}

// ToolResults is what a harness's transcript reader found, with how far its
// read reached, so a row the read could not see is never called "not found".
type ToolResults struct {
	Samples []ToolResultSample
	// Readable is false when no transcript could be read (none exists, none is
	// supported, an error or an expired deadline): every row is then
	// unverified.
	Readable bool
	// Since is the earliest moment the read covers. A row logged before it is
	// outside the coverage. Zero means no lower bound is known.
	Since time.Time
	// Truncated is true when a size, file or time cap cut the read short. A row
	// not found in a truncated read is unverified, not "not found".
	Truncated bool
}

// ReadToolResultsFunc looks for the given CCR ids in the tool results the
// model received. The read is bounded, in memory only and fails open.
type ReadToolResultsFunc func(ids []string) ToolResults
