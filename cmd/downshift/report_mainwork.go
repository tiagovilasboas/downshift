// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"io"
	"time"

	"github.com/tiagovilasboas/downshift-ai/internal/mainwork"
)

// mainWorkRows reads the main-session work log and summarizes it per harness.
// An unreadable log is empty: the rows then show zero for observed harnesses.
func mainWorkRows(days int, now time.Time) []mainwork.HarnessRow {
	var recs []mainwork.Record
	if p, err := mainwork.Path(); err == nil {
		recs, _ = mainwork.Read(p)
	}
	return mainwork.Summarize(recs, days, now)
}

// printMainWork writes one line per harness: the read, search and shell calls
// the main session made itself, and how many ran on a frontier model. Rows are
// never summed. A harness that sends no per-tool event shows "unobserved".
func printMainWork(w io.Writer, rows []mainwork.HarnessRow, scope string) {
	fmt.Fprintf(w, "\nMAIN SESSION MECHANICAL WORK (%s; read/search/shell calls a small-tier subagent could take, per harness, never added)\n", scope)
	for _, r := range rows {
		fmt.Fprintf(w, "   %-14s %s\n", r.Harness, mainWorkText(r))
		if line := readBucketsText(r); line != "" {
			fmt.Fprintf(w, "   %-14s %s\n", "", line)
		}
	}
}

// readBucketsText renders the share of Read results per size class, or "" when
// no Read result size was recorded.
func readBucketsText(r mainwork.HarnessRow) string {
	var total int64
	for _, n := range r.ReadBuckets {
		total += n
	}
	if total == 0 {
		return ""
	}
	var bytes int64
	for _, n := range r.ReadBytes {
		bytes += n
	}
	out := fmt.Sprintf("read results by size (%d results, %d bytes), results%%/bytes%%:", total, bytes)
	for _, b := range mainwork.Buckets {
		bp := int64(0)
		if bytes > 0 {
			bp = r.ReadBytes[b] * 100 / bytes
		}
		out += fmt.Sprintf(" %s %d/%d", b, r.ReadBuckets[b]*100/total, bp)
	}
	return out
}

// mainWorkText renders "main session: 40 read/search/shell calls, 32 on a
// frontier model", or "unobserved" for a harness without the event.
func mainWorkText(r mainwork.HarnessRow) string {
	if !r.Observed {
		return "unobserved"
	}
	return fmt.Sprintf("main session: %d read/search/shell calls, %d on a frontier model", r.Mechanical, r.Frontier)
}
