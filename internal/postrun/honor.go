// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package postrun

import (
	"time"

	"github.com/tiagovilasboas/downshift-ai/internal/hookport"
)

// HonorRow is the part of one post-run exec row the honor check joins on: the
// CCR id it kept ("" when raw output was not stored), the bytes it emitted to
// the model and when it was logged (zero when unknown).
type HonorRow struct {
	KeptID       string
	EmittedBytes int64
	At           time.Time
}

// HonorCounts says, per row, whether the harness showed the model the replaced
// output. Counts only: no text, id or path.
type HonorCounts struct {
	Honored    int // a transcript result names the id and has the emitted length
	NotFound   int // transcripts were read and none names the id
	Mismatch   int // a result names the id, but never at the emitted length
	Unverified int // no join key (no kept id) or no transcript to read
}

// Total is the number of rows classified.
func (c HonorCounts) Total() int { return c.Honored + c.NotFound + c.Mismatch + c.Unverified }

// Add returns the sum of c and o.
func (c HonorCounts) Add(o HonorCounts) HonorCounts {
	return HonorCounts{
		Honored:    c.Honored + o.Honored,
		NotFound:   c.NotFound + o.NotFound,
		Mismatch:   c.Mismatch + o.Mismatch,
		Unverified: c.Unverified + o.Unverified,
	}
}

// ClassifyHonor joins rows to the tool results read returns, in row order.
//
// A row is honored when a transcript result names its kept id and has the
// emitted length, or one byte less (a transcript drops one trailing newline).
// The id alone proves nothing: the pre-run wrapper prints the same retrieve
// hint, so the length has to agree too. Each result honors at most one row:
// two rows that kept the same output need two results. A row takes the one-byte
// shorter result first (the usual shape), then the exact length; when lengths
// collide, the earlier row wins and a later one finds the result used.
//
// A row is unverified, never failed, when it has no kept id or no emitted
// bytes, when there is no reader or no readable transcript, when it was logged
// before the read's coverage, and, when the read was cut short by a cap, when
// its id is simply not found.
func ClassifyHonor(rows []HonorRow, read hookport.ReadToolResultsFunc) HonorCounts {
	var out HonorCounts
	var ids []string
	seen := map[string]bool{}
	for _, r := range rows {
		if verifiable(r) && !seen[r.KeptID] {
			seen[r.KeptID] = true
			ids = append(ids, r.KeptID)
		}
	}
	var res hookport.ToolResults
	if read != nil && len(ids) > 0 {
		res = read(ids)
	}
	avail := map[string]map[int]int{}
	for _, s := range res.Samples {
		if avail[s.ID] == nil {
			avail[s.ID] = map[int]int{}
		}
		avail[s.ID][s.Bytes] += max(s.Count, 1)
	}
	for _, r := range rows {
		switch {
		case !verifiable(r) || !res.Readable || outsideCoverage(r, res):
			out.Unverified++
		case takeResult(avail[r.KeptID], r.EmittedBytes):
			out.Honored++
		case len(avail[r.KeptID]) > 0:
			out.Mismatch++
		case res.Truncated:
			out.Unverified++
		default:
			out.NotFound++
		}
	}
	return out
}

// verifiable reports whether a row has a join key and a length to join on.
func verifiable(r HonorRow) bool { return r.KeptID != "" && r.EmittedBytes > 0 }

// outsideCoverage reports whether the read cannot have seen row r.
func outsideCoverage(r HonorRow, res hookport.ToolResults) bool {
	return !res.Since.IsZero() && (r.At.IsZero() || r.At.Before(res.Since))
}

// takeResult uses up one result of emitted-1 bytes, else one of emitted bytes.
// It reports whether it found one.
func takeResult(byLen map[int]int, emitted int64) bool {
	for _, n := range []int{int(emitted) - 1, int(emitted)} {
		if byLen[n] > 0 {
			byLen[n]--
			return true
		}
	}
	return false
}
