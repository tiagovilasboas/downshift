// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift-ai/internal/execwrap"
	"github.com/tiagovilasboas/downshift-ai/internal/hookport"
	"github.com/tiagovilasboas/downshift-ai/internal/postrun"
)

const (
	prIDA = "0123456789ab"
	prIDB = "ba9876543210"
)

func postRow(kept string, emitted int64) execwrap.Event {
	return execwrap.Event{Entry: postrun.Entry, Harness: "claude-code", Compacted: true, Kept: kept, EmittedBytes: emitted}
}

func fixedReader(samples ...hookport.ToolResultSample) func(string) hookport.ReadToolResultsFunc {
	return func(harness string) hookport.ReadToolResultsFunc {
		if harness != "claude-code" {
			return nil
		}
		return func([]string) ([]hookport.ToolResultSample, bool) { return samples, true }
	}
}

func runPostRunCheck(rows []execwrap.Event, readerFor func(string) hookport.ReadToolResultsFunc) *doctorReport {
	rep := &doctorReport{}
	checkPostRunReplacement(rep, rows, readerFor)
	return rep
}

func TestDoctorPostRunNoRowsAddsNoRow(t *testing.T) {
	rep := runPostRunCheck([]execwrap.Event{{Entry: "hook", Compacted: true, Kept: prIDA}}, fixedReader())
	if len(rep.Checks) != 0 {
		t.Fatalf("checks %+v, want none", rep.Checks)
	}
}

func TestDoctorPostRunCountsAndOK(t *testing.T) {
	rows := []execwrap.Event{
		postRow(prIDA, 100), postRow(prIDB, 200), postRow("", 300),
		{Entry: postrun.Entry, Harness: "claude-code", Compacted: false, Kept: prIDA}, // not offered: left alone
	}
	rep := runPostRunCheck(rows, fixedReader(hookport.ToolResultSample{ID: prIDA, Bytes: 99}))
	if len(rep.Checks) != 1 || rep.Warnings != 0 {
		t.Fatalf("checks %+v warnings %d", rep.Checks, rep.Warnings)
	}
	c := rep.Checks[0]
	if c.ID != "post-run replacement" || c.Status != doctorOK {
		t.Fatalf("check %+v", c)
	}
	for _, want := range []string{"honored 1", "not found 1", "mismatch 0", "unverified 1", "offered"} {
		if !strings.Contains(c.Detail, want) {
			t.Errorf("detail %q lacks %q", c.Detail, want)
		}
	}
}

func TestDoctorPostRunWarnsWhenNoneHonored(t *testing.T) {
	rows := []execwrap.Event{postRow(prIDA, 100), postRow("", 300)}
	rep := runPostRunCheck(rows, fixedReader())
	if rep.Warnings != 1 || rep.Checks[0].Status != doctorWarn || rep.Checks[0].Fix == "" {
		t.Fatalf("want one warn with a fix, got %+v", rep.Checks)
	}
}

func TestDoctorPostRunHarnessWithoutReaderIsUnverifiedAndWarns(t *testing.T) {
	row := postRow(prIDA, 100)
	row.Harness = "other"
	rep := runPostRunCheck([]execwrap.Event{row}, fixedReader(hookport.ToolResultSample{ID: prIDA, Bytes: 100}))
	if !strings.Contains(rep.Checks[0].Detail, "unverified 1") || rep.Checks[0].Status != doctorWarn {
		t.Fatalf("check %+v", rep.Checks[0])
	}
}

func TestDoctorPostRunLooksOnlyAtRecentRows(t *testing.T) {
	var rows []execwrap.Event
	rows = append(rows, postRow(prIDA, 100)) // oldest, beyond the window
	for i := 0; i < postRunDoctorWindow; i++ {
		rows = append(rows, postRow("", 1))
	}
	rep := runPostRunCheck(rows, fixedReader(hookport.ToolResultSample{ID: prIDA, Bytes: 100}))
	if !strings.Contains(rep.Checks[0].Detail, "honored 0") {
		t.Fatalf("the oldest row must be outside the window: %q", rep.Checks[0].Detail)
	}
}
