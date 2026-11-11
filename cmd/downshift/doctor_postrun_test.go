// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"strings"
	"testing"
	"time"

	"github.com/tiagovilasboas/downshift-ai/internal/execwrap"
	"github.com/tiagovilasboas/downshift-ai/internal/hookport"
	"github.com/tiagovilasboas/downshift-ai/internal/postrun"
)

const (
	prIDA = "0123456789ab"
	prIDB = "ba9876543210"
)

var prNow = time.Date(2026, 11, 11, 12, 0, 0, 0, time.UTC)

func postRowFor(harness, kept string, emitted int64) execwrap.Event {
	return execwrap.Event{Entry: postrun.Entry, Harness: harness, Compacted: true, Kept: kept,
		EmittedBytes: emitted, TS: prNow.Format(time.RFC3339)}
}

func postRow(kept string, emitted int64) execwrap.Event {
	return postRowFor("claude-code", kept, emitted)
}

// readerFor gives each named harness a reader that returns samples, and every
// other harness none.
func readerFor(samples map[string][]hookport.ToolResultSample) func(string) hookport.ReadToolResultsFunc {
	return func(harness string) hookport.ReadToolResultsFunc {
		s, ok := samples[harness]
		if !ok {
			return nil
		}
		return func([]string) hookport.ToolResults { return hookport.ToolResults{Readable: true, Samples: s} }
	}
}

func claudeSamples(s ...hookport.ToolResultSample) func(string) hookport.ReadToolResultsFunc {
	return readerFor(map[string][]hookport.ToolResultSample{"claude-code": s})
}

func runPostRunCheck(rows []execwrap.Event, rf func(string) hookport.ReadToolResultsFunc) *doctorReport {
	rep := &doctorReport{}
	checkPostRunReplacement(rep, rows, rf)
	return rep
}

func TestDoctorPostRunNoRowsAddsNoRow(t *testing.T) {
	rep := runPostRunCheck([]execwrap.Event{{Entry: "hook", Compacted: true, Kept: prIDA}}, claudeSamples())
	if len(rep.Checks) != 0 {
		t.Fatalf("checks %+v, want none", rep.Checks)
	}
}

func TestDoctorPostRunCountsAndOK(t *testing.T) {
	rows := []execwrap.Event{
		postRow(prIDA, 100), postRow(prIDB, 200), postRow("", 300),
		{Entry: postrun.Entry, Harness: "claude-code", Compacted: false, Kept: prIDA}, // not offered: left alone
	}
	rep := runPostRunCheck(rows, claudeSamples(hookport.ToolResultSample{ID: prIDA, Bytes: 99, Count: 1}))
	if len(rep.Checks) != 1 || rep.Warnings != 0 {
		t.Fatalf("checks %+v warnings %d", rep.Checks, rep.Warnings)
	}
	c := rep.Checks[0]
	if c.ID != "post-run replacement" || c.Harness != "claude-code" || c.Status != doctorOK {
		t.Fatalf("check %+v", c)
	}
	for _, want := range []string{"honored 1", "not found 1", "mismatch 0", "unverified 1", "offered"} {
		if !strings.Contains(c.Detail, want) {
			t.Errorf("detail %q lacks %q", c.Detail, want)
		}
	}
}

func TestDoctorPostRunWarnsWhenCheckedRowsAreNoneHonored(t *testing.T) {
	rep := runPostRunCheck([]execwrap.Event{postRow(prIDA, 100), postRow("", 300)}, claudeSamples())
	if rep.Warnings != 1 || rep.Checks[0].Status != doctorWarn || rep.Checks[0].Fix == "" {
		t.Fatalf("want one warn with a fix, got %+v", rep.Checks)
	}
}

func TestDoctorPostRunUnverifiedAloneIsNotAFailure(t *testing.T) {
	// A harness with no transcript reader, and a row with no kept id.
	other := postRowFor("other", prIDA, 100)
	rep := runPostRunCheck([]execwrap.Event{other, postRow("", 5)}, claudeSamples())
	if rep.Warnings != 0 || len(rep.Checks) != 2 {
		t.Fatalf("warnings %d, checks %+v", rep.Warnings, rep.Checks)
	}
	for _, c := range rep.Checks {
		if c.Status != doctorOK || c.Fix != "" || !strings.Contains(c.Detail, "honored 0") {
			t.Errorf("check %+v", c)
		}
	}
}

func TestDoctorPostRunReportsPerHarness(t *testing.T) {
	// One harness honored, another did not: pooling would hide the second.
	rows := []execwrap.Event{
		postRowFor("claude-code", prIDA, 100),
		postRowFor("zeta", prIDB, 100),
	}
	rf := readerFor(map[string][]hookport.ToolResultSample{
		"claude-code": {{ID: prIDA, Bytes: 99, Count: 1}},
		"zeta":        {},
	})
	rep := runPostRunCheck(rows, rf)
	if len(rep.Checks) != 2 || rep.Warnings != 1 {
		t.Fatalf("checks %+v warnings %d", rep.Checks, rep.Warnings)
	}
	if rep.Checks[0].Harness != "claude-code" || rep.Checks[0].Status != doctorOK ||
		rep.Checks[1].Harness != "zeta" || rep.Checks[1].Status != doctorWarn {
		t.Fatalf("checks %+v", rep.Checks)
	}
}

func TestDoctorPostRunLooksOnlyAtRecentRows(t *testing.T) {
	rows := []execwrap.Event{postRow(prIDA, 100)} // oldest, beyond the window
	for i := 0; i < postRunDoctorWindow; i++ {
		rows = append(rows, postRow("", 1))
	}
	rep := runPostRunCheck(rows, claudeSamples(hookport.ToolResultSample{ID: prIDA, Bytes: 100, Count: 1}))
	if !strings.Contains(rep.Checks[0].Detail, "honored 0") {
		t.Fatalf("the oldest row must be outside the window: %q", rep.Checks[0].Detail)
	}
}
