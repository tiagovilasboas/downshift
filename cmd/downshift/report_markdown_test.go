// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"strings"
	"testing"
	"time"

	"github.com/tiagovilasboas/downshift-ai/internal/execwrap"
)

// --markdown carries the text report's figures as headings and tables, per
// harness, with no money and no cross-harness total.
func TestReport_Markdown(t *testing.T) {
	belowFixtureEnv(t)
	md := runReportOut(t, "--days=0", "--markdown")
	for _, want := range []string{
		"# Downshift report (all time)",
		"## Compaction savings per harness",
		"## Routed spawns (all time)",
		"| harness | spawns | ended | completed | billable/completed | rework |",
		"| claude-code | 3 | 3 | 3 |",
		"| codex | 1 | 1 | 1 |",
		"## Routing: tokens on a lighter model, per harness",
		"| harness | subagent tokens | on a lighter model | share | moved by Downshift |",
		"| claude-code | 2.2k | 1.7k (1 of 2 agents) | 77% |",
		"(0 guardrail) | 77% |", // the summary's cheaper-model share matches the routing section
		"| codex | unobserved | - | - | - |",
		"- **codex**: 1 agent unobserved (no model could be ranked)",
		"## Why spawns changed",
		"## Main session mechanical work",
		"## Quota used per harness",
	} {
		if !strings.Contains(md, want) {
			t.Errorf("markdown lacks %q:\n%s", want, md)
		}
	}
	if strings.Contains(md, "$") || strings.Contains(strings.ToLower(md), "total") {
		t.Errorf("markdown must carry no money and no total:\n%s", md)
	}
}

// With no format flag the report is markdown, and the Compactor section breaks
// each harness down by family and by why a command was not compacted or wrapped.
func TestReport_MarkdownIsDefaultWithCompactionDetail(t *testing.T) {
	whyFixtureEnv(t)
	seedCompactorAndQuota(t)
	skips, err := execwrap.SkipsPath()
	if err != nil {
		t.Fatal(err)
	}
	if err := execwrap.AppendSkip(skips, execwrap.SkipMetachar, "claude-code", time.Now()); err != nil {
		t.Fatal(err)
	}
	md := runReportOut(t, "--days=0")
	for _, want := range []string{
		"# Downshift report (all time)",
		"### claude-code: 1 commands, 1 compacted",
		"| family | commands | compacted | output tokens | tokens avoided |",
		"| other | 1 | 1 | 1.1k | 1.0k |",
		"**Left unwrapped by the hook**",
		"| pipe, list, quote or redirect | 1 |",
	} {
		if !strings.Contains(md, want) {
			t.Errorf("default report lacks %q:\n%s", want, md)
		}
	}
	if strings.Contains(md, "$") || strings.Contains(strings.ToLower(md), "total") {
		t.Errorf("markdown must carry no money and no total:\n%s", md)
	}
	if text := runReportOut(t, "--days=0", "--text"); !strings.Contains(text, "COMPACTION DETAIL PER HARNESS") {
		t.Errorf("--text lacks the compaction detail:\n%s", text)
	}
}

// A saving the agent read back with `retrieve` is subtracted in every view, and
// the default markdown report says how many bytes were read back.
func TestReport_ReadBackBytesAreSubtractedAndShown(t *testing.T) {
	whyFixtureEnv(t)
	path, err := execwrap.EventsPath()
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Format(time.RFC3339)
	const id = "0123456789ab"
	for _, ev := range []execwrap.Event{
		{TS: now, Harness: "claude-code", RawBytes: 4400, EmittedBytes: 400, Compacted: true, Kept: id},
		{TS: now, Harness: "claude-code", RawBytes: 2400, EmittedBytes: 400, Compacted: true},
	} {
		if err := execwrap.Append(path, ev); err != nil {
			t.Fatal(err)
		}
	}
	execwrap.RecordRetrieval(id, 1000, time.Now())

	md := runReportOut(t, "--days=0", "--markdown")
	// 4000 + 2000 saved, 1000 read back: 5000 bytes, 1250 tokens.
	if !strings.Contains(md, "| claude-code | 1.2k |") && !strings.Contains(md, "| claude-code | 1.3k |") {
		t.Errorf("markdown does not show the net 1250 tokens:\n%s", md)
	}
	if !strings.Contains(md, "Read back with `retrieve`: 1000 bytes") {
		t.Errorf("markdown does not say how much was read back:\n%s", md)
	}
	text := runReportOut(t, "--days=0", "--text")
	if !strings.Contains(text, "Read back with retrieve: 1000 bytes") {
		t.Errorf("text report does not say how much was read back:\n%s", text)
	}
}
