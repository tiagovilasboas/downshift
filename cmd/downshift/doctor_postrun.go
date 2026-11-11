// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"sort"
	"time"

	"github.com/tiagovilasboas/downshift-ai/internal/execwrap"
	"github.com/tiagovilasboas/downshift-ai/internal/hookport"
	"github.com/tiagovilasboas/downshift-ai/internal/postrun"
)

// postRunDoctorWindow is how many of the latest post-run rows the doctor row
// looks at.
const postRunDoctorWindow = 200

// checkPostRunReplacementFromLog reads the exec log and reports on it. A
// missing or unreadable log adds no row.
func checkPostRunReplacementFromLog(rep *doctorReport) {
	path, err := execwrap.EventsPath()
	if err != nil {
		return
	}
	rows, _ := execwrap.ReadEvents(path)
	checkPostRunReplacement(rep, rows, func(harness string) hookport.ReadToolResultsFunc {
		return portFor(harness).ReadToolResults
	})
}

// checkPostRunReplacement tells, per harness, whether it used what the
// post-run hook offered. Each recent compacted post-run row is joined, by kept
// id and emitted length, to the tool results the harness's transcript says the
// model received (postrun.ClassifyHonor). Harness-free: the transcript reader
// comes from the harness's port, and a harness without one gets its rows
// classified as unverified.
//
// A harness row warns only when it has rows that could be checked and none was
// honored (honored 0 with some not found or mismatch). Unverified rows alone
// are never a failure. Counts are never pooled across harnesses, so one
// harness's honored rows cannot hide another's. No post-run rows adds no row.
func checkPostRunReplacement(rep *doctorReport, events []execwrap.Event, readerFor func(string) hookport.ReadToolResultsFunc) {
	var rows []execwrap.Event
	for _, ev := range events {
		if ev.Entry == postrun.Entry && ev.Compacted {
			rows = append(rows, ev)
		}
	}
	if len(rows) > postRunDoctorWindow {
		rows = rows[len(rows)-postRunDoctorWindow:]
	}
	byHarness := map[string][]postrun.HonorRow{}
	for _, ev := range rows {
		at, _ := time.Parse(time.RFC3339, ev.TS) // zero when unparsable: unverified
		byHarness[ev.Harness] = append(byHarness[ev.Harness], postrun.HonorRow{KeptID: ev.Kept, EmittedBytes: ev.EmittedBytes, At: at})
	}
	harnesses := make([]string, 0, len(byHarness))
	for h := range byHarness {
		harnesses = append(harnesses, h)
	}
	sort.Strings(harnesses)
	for _, h := range harnesses {
		n := postrun.ClassifyHonor(byHarness[h], readerFor(h))
		name := h
		if name == "" {
			name = "unknown"
		}
		c := doctorCheck{ID: "post-run replacement", Harness: h, Status: doctorOK, Detail: fmt.Sprintf(
			"%s: %d recent rows: honored %d, not found %d, mismatch %d, unverified %d (their savings are offered to the model, not proven used)",
			name, n.Total(), n.Honored, n.NotFound, n.Mismatch, n.Unverified)}
		if n.Honored == 0 && n.NotFound+n.Mismatch > 0 {
			c.Status = doctorWarn
			c.Fix = "check that the harness honors the PostToolUse updatedToolOutput answer (docs/design/post-run-compaction.md), or set DOWNSHIFT_POST_COMPACT=0 to stop offering replaced output"
		}
		rep.add(c)
	}
}
