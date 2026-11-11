// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"

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

// checkPostRunReplacement tells whether the harness used what the post-run
// hook offered. Each recent compacted post-run row is joined, by kept id and
// emitted length, to the tool results the harness's transcript says the model
// received (postrun.ClassifyHonor). The row warns only when rows exist and none
// could be honored. No post-run rows adds no row. Harness-free: the
// transcript reader comes from the harness's port.
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
	if len(rows) == 0 {
		return
	}
	byHarness := map[string][]postrun.HonorRow{}
	for _, ev := range rows {
		byHarness[ev.Harness] = append(byHarness[ev.Harness], postrun.HonorRow{KeptID: ev.Kept, EmittedBytes: ev.EmittedBytes})
	}
	var total postrun.HonorCounts
	for harness, hr := range byHarness {
		total = total.Add(postrun.ClassifyHonor(hr, readerFor(harness)))
	}
	c := doctorCheck{ID: "post-run replacement", Status: doctorOK, Detail: fmt.Sprintf(
		"%d recent rows: honored %d, not found %d, mismatch %d, unverified %d (their savings are offered to the model, not proven used)",
		total.Total(), total.Honored, total.NotFound, total.Mismatch, total.Unverified)}
	if total.Honored == 0 {
		c.Status = doctorWarn
		c.Fix = "check that the harness honors the PostToolUse updatedToolOutput answer (docs/design/post-run-compaction.md), or set DOWNSHIFT_POST_COMPACT=0 to stop offering replaced output"
	}
	rep.add(c)
}
