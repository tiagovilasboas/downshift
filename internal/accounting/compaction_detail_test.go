// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package accounting

import (
	"reflect"
	"testing"
	"time"

	"github.com/tiagovilasboas/downshift-ai/internal/execwrap"
)

func TestBuildCompactionDetail(t *testing.T) {
	now := time.Date(2026, 11, 3, 12, 0, 0, 0, time.UTC)
	recent := now.Add(-time.Hour).Format(time.RFC3339)
	old := now.Add(-40 * 24 * time.Hour).Format(time.RFC3339)
	execEvents := []execwrap.Event{
		{TS: recent, Harness: "claude-code", Family: "go-test", RawBytes: 4000, EmittedBytes: 400, Compacted: true},
		{TS: recent, Harness: "claude-code", Family: "go-test", RawBytes: 800, EmittedBytes: 800, Passthrough: execwrap.PassthroughSmallOutput},
		{TS: recent, Harness: "claude-code", Family: "git-diff", RawBytes: 8000, EmittedBytes: 8000, Passthrough: execwrap.PassthroughUnrecognized},
		{TS: recent, Harness: "claude-code", Family: "git-status", RawBytes: 40, EmittedBytes: 40},
		{TS: recent, Family: "make", RawBytes: 400, EmittedBytes: 400, Passthrough: execwrap.PassthroughNonZeroExit},
		{TS: old, Harness: "claude-code", Family: "go-test", RawBytes: 99999, EmittedBytes: 1, Compacted: true},
	}
	skips := []execwrap.SkipEvent{
		{TS: recent, Reason: execwrap.SkipMetachar, Harness: "claude-code"},
		{TS: recent, Reason: execwrap.SkipMetachar, Harness: "claude-code"},
		{TS: recent, Reason: "too_long", Harness: "claude-code"},
		{TS: recent, Reason: execwrap.SkipMetachar, Harness: "kirocrew"},
		{TS: old, Reason: execwrap.SkipNotAllowlisted, Harness: "claude-code"},
		{TS: "not a time", Reason: execwrap.SkipDenied, Harness: "claude-code"},
	}

	got := BuildCompactionDetail(execEvents, skips, Scope{Kind: ScopeDays, Days: 30}, now)

	want := CompactionDetail{Harnesses: []HarnessCompaction{
		{
			Harness: "claude-code", Commands: 4, Compacted: 1,
			Families: []FamilyCompaction{
				{Family: "git-diff", Commands: 1, RawTokens: 2000},
				{Family: "go-test", Commands: 2, Compacted: 1, RawTokens: 1200, AvoidedTokens: 900},
				{Family: "git-status", Commands: 1, RawTokens: 10},
			},
			NotCompacted: []ReasonCount{
				{Reason: execwrap.PassthroughNotRecorded, Count: 1},
				{Reason: execwrap.PassthroughSmallOutput, Count: 1},
				{Reason: execwrap.PassthroughUnrecognized, Count: 1},
			},
			Skipped: []ReasonCount{{Reason: execwrap.SkipMetachar, Count: 2}, {Reason: "too_long", Count: 1}},
		},
		{
			Harness: UnknownHarness, Commands: 1,
			Families:     []FamilyCompaction{{Family: "make", Commands: 1, RawTokens: 100}},
			NotCompacted: []ReasonCount{{Reason: execwrap.PassthroughNonZeroExit, Count: 1}},
			Skipped:      []ReasonCount{},
		},
		{
			Harness: "kirocrew", Families: []FamilyCompaction{}, NotCompacted: []ReasonCount{},
			Skipped: []ReasonCount{{Reason: execwrap.SkipMetachar, Count: 1}},
		},
	}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("detail mismatch\n got: %+v\nwant: %+v", got, want)
	}
}

// Skip rows hold no session, so a session scope keeps none of them.
func TestBuildCompactionDetail_SessionScopeHasNoSkips(t *testing.T) {
	now := time.Now()
	ts := now.Format(time.RFC3339)
	got := BuildCompactionDetail(
		[]execwrap.Event{{TS: ts, Session: "abc", Harness: "claude-code", Family: "go-test", RawBytes: 40, EmittedBytes: 40}},
		[]execwrap.SkipEvent{{TS: ts, Reason: execwrap.SkipMetachar, Harness: "claude-code"}},
		Scope{Kind: ScopeSession, SessionHash: "abc"}, now)
	if len(got.Harnesses) != 1 || got.Harnesses[0].Commands != 1 || len(got.Harnesses[0].Skipped) != 0 {
		t.Fatalf("session scope: %+v", got)
	}
}
