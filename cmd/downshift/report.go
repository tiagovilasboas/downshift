// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/tiagovilasboas/downshift-ai/internal/accounting"
	"github.com/tiagovilasboas/downshift-ai/internal/execwrap"
	"github.com/tiagovilasboas/downshift-ai/internal/mainwork"
	"github.com/tiagovilasboas/downshift-ai/internal/telemetry"
)

// defaultReportDays is the window `downshift report` uses without --days. The
// Agents panel uses the same window, so both read this one constant.
const defaultReportDays = 30

// runReport prints the routed spawns: billable tokens per completed task, the
// share on a lower tier, and the rework rate and rework per verdict. It only
// reads the event log.
func runReport(args []string, stdout, stderr io.Writer) int {
	if len(args) > 0 && args[0] == "clear" {
		return runReportClear(args[1:], os.Stdin, stdout, stderr, time.Now())
	}
	days := defaultReportDays
	asJSON, asText := false, false
	var periods periodsRequest
	daily, history, daysSet := false, false, false
	for _, arg := range args {
		switch {
		case arg == "--daily":
			daily = true
		case arg == "--history":
			history = true
		case strings.HasPrefix(arg, "--by="):
			periods.per = strings.TrimPrefix(arg, "--by=")
		case arg == "--json":
			asJSON = true
		case arg == "--text":
			asText = true
		case arg == "--markdown":
			// The default since the report became markdown; kept so old calls work.
		case strings.HasPrefix(arg, "--days="):
			n, err := strconv.ParseFloat(strings.TrimPrefix(arg, "--days="), 64)
			if err != nil || n < 0 {
				fmt.Fprintf(stderr, "invalid --days value: %s\n", strings.TrimPrefix(arg, "--days="))
				return 2
			}
			days, daysSet = int(n), true
		default:
			fmt.Fprintf(stderr, "unknown flag: %s\nusage: downshift report [--days=N] [--json|--text] [--daily | --history [--by=day|week]]\n", arg)
			return 2
		}
	}
	if daily || history || periods.per != "" {
		req, msg := resolvePeriods(daily, history, periods.per, days, daysSet)
		if msg != "" {
			fmt.Fprintln(stderr, msg)
			return 2
		}
		return runReportPeriods(req, asJSON, stdout, stderr, time.Now())
	}

	events, err := telemetry.ReadEvents()
	if err != nil {
		fmt.Fprintf(stderr, "error reading event log: %v\n", err)
		return 1
	}

	// The exec log is read as a whole and windowed by accounting.Compute, so the
	// routing and the Compactor share the --days window.
	execEvents, _ := accounting.ReadExecEvents()
	nums := accounting.Compute(events, execEvents, accounting.Scope{Kind: accounting.ScopeDays, Days: days})

	scope := accounting.Scope{Kind: accounting.ScopeDays, Days: days}
	data := buildReportOut(events, execEvents, nums, days, scope)

	if styledReportWanted && !asJSON && !asText {
		if err := showStyledReport(styledReportData(data, events, days, scope)); err == nil {
			return 0
		}
	}
	if asJSON {
		out, err := json.MarshalIndent(data, "", "  ")
		if err != nil {
			fmt.Fprintf(stderr, "error encoding report: %v\n", err)
			return 1
		}
		fmt.Fprintln(stdout, string(out))
		return 0
	}

	if !asText {
		printReportMarkdown(stdout, data, scope.Label())
		return 0
	}

	// The Compactor leads: it runs on every harness and OS with no
	// model-rewrite channel, so it carries no platform risk. Per harness, in
	// tokens, from the same exec-log rows the routing table reuses below.
	printSummaryText(stdout, data.Summary, scope.Label())

	// The routed row, the rework per verdict, the plan lens and the per-harness
	// completion lines are the canonical report; telemetry owns their wording
	// (the tests read those strings), so delegate instead of reprinting them.
	telemetry.PrintSegmentedReport(stdout, events, days)

	// Compactor savings are produced outside the event log, so they are
	// appended here rather than inside telemetry.PrintReport.
	fmt.Fprintf(stdout, "\n📦 COMPACTOR (%s)\n", nums.Scope.Label())
	fmt.Fprintf(stdout, "   %s\n", nums.Display.Compactor)
	printCompactionDetailText(stdout, data.CompactionDetail)
	printEfficiencyText(stdout, data.Efficiency)

	// The routing note and the quota per harness: added sections, built from
	// the same events and window, with no money in either.
	printRoutingWhy(stdout, data.WhySpawnsChanged)
	printMainWork(stdout, data.MainSessionWork, scope.Label())
	printQuotaByHarness(stdout, data.TokensByHarness, scope)
	fmt.Fprintf(stdout, "\n%s\n", accounting.AdoptionLine(data.Adoption))

	return 0
}

// reportOut is the `--json` report: the comparison and its keys as before,
// plus the Numbers every surface shares.
type reportOut struct {
	telemetry.Report
	// Summary is the head of the report: what the agents pane shows, for the window.
	Summary        []paneRow                 `json:"summary_by_harness"`
	Numbers        accounting.Numbers        `json:"numbers"`
	CurrentHarness string                    `json:"current_harness,omitempty"`
	ByHarness      []telemetry.HarnessReport `json:"by_harness,omitempty"`
	// WhySpawnsChanged is the routing note per harness; TokensByHarness is, per
	// harness, the routing tokens and the quota used (never summed).
	WhySpawnsChanged accounting.RoutingWhy      `json:"why_spawns_changed"`
	TokensByHarness  []accounting.HarnessTokens `json:"tokens_by_harness"`
	// CompactionByHarness is the leading section's data: per harness, the
	// Compactor tokens only (tokens, no money). Printed once, here.
	CompactionByHarness []compactionRow `json:"compaction_by_harness"`
	// CompactionDetail is, per harness, the Compactor by family, why runs were
	// not compacted and why the hook left commands unwrapped.
	CompactionDetail accounting.CompactionDetail `json:"compaction_detail"`
	// MainSessionWork is the mechanical work the main session did itself, one
	// row per harness (observed false: the harness sends no per-tool event).
	MainSessionWork []mainwork.HarnessRow `json:"main_session_work"`
	// Efficiency is, per harness, the premium capacity preserved, the spawns by
	// the tier they ran on and the rework (counts and shares, no tokens, no money).
	Efficiency accounting.Efficiency `json:"efficiency"`
	// Adoption is the MCP plan and delegate calls over the window (counts only).
	Adoption accounting.Adoption `json:"adoption"`
}

// buildReportOut assembles every section of the report from the logs, for
// days and scope. Pure apart from reading the quota and main-work files.
func buildReportOut(events []telemetry.Event, execEvents []execwrap.Event, nums accounting.Numbers, days int, scope accounting.Scope) reportOut {
	rep := telemetry.BuildReport(events, days)
	rep.SetCompactor(nums.Compactor.SavedBytes, nums.Compactor.Tokens)
	currentHarness, segments := telemetry.SegmentReports(events, days)
	why := accounting.BuildRoutingWhy(events, scope)
	mainWork := mainWorkRows(days, time.Now())
	skips, _ := accounting.ReadSkipEvents()
	detail := accounting.BuildCompactionDetail(execEvents, skips, scope, time.Now())
	efficiency := accounting.BuildEfficiency(events, scope)
	perHarness := accounting.TokensByHarness(events, execEvents, scope, quotaByHarness(time.Now()))

	return reportOut{
		Report: rep, Numbers: nums, CurrentHarness: currentHarness, ByHarness: segments,
		WhySpawnsChanged: why, TokensByHarness: perHarness, CompactionByHarness: compactionRows(perHarness),
		MainSessionWork: mainWork, CompactionDetail: detail, Efficiency: efficiency,
		Adoption: accounting.BuildAdoption(events, scope),
		Summary:  buildPaneRows(events, execEvents, scope),
	}
}
