// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"strings"
	"testing"
	"time"

	"github.com/tiagovilasboas/downshift/internal/execwrap"
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
