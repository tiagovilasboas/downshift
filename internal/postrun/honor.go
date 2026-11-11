// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package postrun

import "github.com/tiagovilasboas/downshift-ai/internal/hookport"

// HonorRow is the part of one post-run exec row the honor check joins on: the
// CCR id it kept ("" when raw output was not stored) and the bytes it emitted
// to the model.
type HonorRow struct {
	KeptID       string
	EmittedBytes int64
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

// lengthSlack is how much shorter than emitted a transcript's result may be:
// a transcript drops one trailing newline.
const lengthSlack = 1

// ClassifyHonor joins rows to the tool results read returns. A row needs both
// its kept id and its length to match: the pre-run wrapper prints the same
// marker, so an id alone proves nothing. A nil read, an unreadable transcript
// or a row without a kept id leaves the row unverified, never failed.
func ClassifyHonor(rows []HonorRow, read hookport.ReadToolResultsFunc) HonorCounts {
	var out HonorCounts
	var ids []string
	seen := map[string]bool{}
	for _, r := range rows {
		if r.KeptID != "" && !seen[r.KeptID] {
			seen[r.KeptID] = true
			ids = append(ids, r.KeptID)
		}
	}
	var samples []hookport.ToolResultSample
	readable := false
	if read != nil && len(ids) > 0 {
		samples, readable = read(ids)
	}
	byID := map[string][]int{}
	for _, s := range samples {
		byID[s.ID] = append(byID[s.ID], s.Bytes)
	}
	for _, r := range rows {
		switch {
		case r.KeptID == "" || !readable:
			out.Unverified++
		case len(byID[r.KeptID]) == 0:
			out.NotFound++
		case lengthMatches(byID[r.KeptID], r.EmittedBytes):
			out.Honored++
		default:
			out.Mismatch++
		}
	}
	return out
}

func lengthMatches(got []int, emitted int64) bool {
	for _, n := range got {
		if d := emitted - int64(n); d >= 0 && d <= lengthSlack {
			return true
		}
	}
	return false
}
