// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package mainwork

import (
	"time"

	"github.com/tiagovilasboas/downshift/internal/core"
)

// HarnessRow is one harness's main-session work. Rows are never summed across
// harnesses. Observed false means the harness sends no per-tool event, so the
// counts say nothing (not zero).
type HarnessRow struct {
	Harness    string           `json:"harness"`
	Observed   bool             `json:"observed"`
	Mechanical int64            `json:"mechanical_calls"`
	Frontier   int64            `json:"mechanical_on_frontier"`
	ByCategory map[string]int64 `json:"by_category,omitempty"`
	// ReadBuckets counts Read results per size class (Buckets); calls without
	// a recorded size are left out.
	ReadBuckets map[string]int64 `json:"read_result_buckets,omitempty"`
	// ReadBytes sums the bytes of those results per size class.
	ReadBytes map[string]int64 `json:"read_result_bytes,omitempty"`
}

// Summarize builds one row per harness in core.AllHarnessCaps, over the
// records newer than days (days <= 0 means all).
//
// The tier of a call is the tier of its session's latest recorded parent model
// (telemetry.DirectionCatalog ranks it); a session with no decision, or a model
// the catalog does not know, is not counted as frontier.
func Summarize(recs []Record, days int, now time.Time) []HarnessRow {
	return summarize(recs, days, now, sessionFrontier)
}

func summarize(recs []Record, days int, now time.Time, frontierOf func(recs []Record) map[string]bool) []HarnessRow {
	frontier := frontierOf(recs)
	var cutoff time.Time
	if days > 0 {
		cutoff = now.AddDate(0, 0, -days)
	}
	var rows []HarnessRow
	for _, h := range core.AllHarnessCaps() {
		row := HarnessRow{Harness: h.Harness, Observed: h.Caps.MainWorkObserved()}
		if row.Observed {
			for _, rec := range recs {
				if rec.Harness != h.Harness || !within(rec.TS, cutoff) {
					continue
				}
				if row.ByCategory == nil {
					row.ByCategory = map[string]int64{}
				}
				row.ByCategory[rec.Category]++
				if rec.ResultBytesBucket != "" {
					if row.ReadBuckets == nil {
						row.ReadBuckets = map[string]int64{}
					}
					row.ReadBuckets[rec.ResultBytesBucket]++
					if row.ReadBytes == nil {
						row.ReadBytes = map[string]int64{}
					}
					row.ReadBytes[rec.ResultBytesBucket] += int64(rec.ResultBytes)
				}
				if core.ToolCategory(rec.Category).IsMechanical() {
					row.Mechanical++
					if rec.Tier == core.TierFrontier.String() || frontier[rec.Harness+"/"+rec.Session] {
						row.Frontier++
					}
				}
			}
		}
		rows = append(rows, row)
	}
	return rows
}

func within(ts string, cutoff time.Time) bool {
	if cutoff.IsZero() {
		return true
	}
	t, err := time.Parse(time.RFC3339, ts)
	return err == nil && !t.Before(cutoff)
}
