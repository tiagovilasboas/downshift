// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/tiagovilasboas/downshift-ai/internal/accounting"
	"github.com/tiagovilasboas/downshift-ai/internal/telemetry"
)

// periodsRequest is `report --daily` or `report --history`: one row per day
// (the --days window) or the whole retained history by day or week.
type periodsRequest struct {
	history bool
	per     string // accounting.PerDay or PerWeek
	days    int
}

// periodsOut is the `--json` shape of both views. Tokens only.
type periodsOut struct {
	Per       string                 `json:"per"`
	Days      int                    `json:"days,omitempty"` // 0: the whole history
	Harnesses []string               `json:"harnesses"`
	Rows      []accounting.PeriodRow `json:"rows"`
}

// resolvePeriods validates the flag combination. The second result is the
// usage error, or "" when the request is valid.
func resolvePeriods(daily, history bool, by string, days int, daysSet bool) (periodsRequest, string) {
	switch {
	case daily && history:
		return periodsRequest{}, "--daily and --history cannot be combined"
	case !daily && !history:
		return periodsRequest{}, "--by needs --history"
	case by != "" && daily:
		return periodsRequest{}, "--by needs --history"
	case history && daysSet:
		return periodsRequest{}, "--history covers the whole retained history; drop --days"
	case days <= 0 && daily:
		return periodsRequest{}, "--daily needs --days of at least 1"
	}
	if daily {
		return periodsRequest{per: accounting.PerDay, days: days}, ""
	}
	switch by {
	case "", accounting.PerWeek:
		return periodsRequest{history: true, per: accounting.PerWeek}, ""
	case accounting.PerDay:
		return periodsRequest{history: true, per: accounting.PerDay}, ""
	}
	return periodsRequest{}, "invalid --by value: " + by + " (day or week)"
}

// runReportPeriods prints the per-period view. It only reads the logs.
func runReportPeriods(req periodsRequest, asJSON bool, stdout, stderr io.Writer, now time.Time) int {
	events, err := telemetry.ReadEvents()
	if err != nil {
		fmt.Fprintf(stderr, "error reading event log: %v\n", err)
		return 1
	}
	execEvents, _ := accounting.ReadExecEvents()
	opts := accounting.PeriodOptions{Per: req.per, Now: now, Loc: now.Location()}
	if !req.history {
		opts.Days, opts.Fill = req.days, true
	}
	out := periodsOut{Per: req.per, Days: opts.Days}
	out.Rows = accounting.BuildPeriods(events, execEvents, opts)
	out.Harnesses = periodHarnessNames(out.Rows)
	if asJSON {
		b, err := json.MarshalIndent(out, "", "  ")
		if err != nil {
			fmt.Fprintf(stderr, "error encoding report: %v\n", err)
			return 1
		}
		fmt.Fprintln(stdout, string(b))
		return 0
	}
	printPeriodsTable(stdout, out)
	return 0
}

func periodHarnessNames(rows []accounting.PeriodRow) []string {
	seen := map[string]bool{}
	names := []string{}
	for _, r := range rows {
		for _, h := range r.Harnesses {
			if !seen[h.Harness] {
				seen[h.Harness] = true
				names = append(names, h.Harness)
			}
		}
	}
	sort.Strings(names)
	return names
}

// printPeriodsTable prints one row per period with two columns per harness:
// tokens the Compactor kept out of the context ("saved") and billable tokens
// that ran below the session model ("below"). The two are never added.
func printPeriodsTable(w io.Writer, out periodsOut) {
	title := "Tokens per day"
	switch {
	case out.Per == accounting.PerWeek:
		title = "Tokens per week (from Monday), whole retained history"
	case out.Days == 0:
		title += ", whole retained history"
	default:
		title += fmt.Sprintf(", last %d days", out.Days)
	}
	fmt.Fprintf(w, "%s\n", title)
	if len(out.Harnesses) == 0 {
		fmt.Fprintln(w, "No compaction or routed spawns in this window.")
		return
	}
	header := []string{out.Per}
	for _, h := range out.Harnesses {
		header = append(header, h+" runs", h+" compacted", h+" saved", h+" below")
	}
	table := [][]string{header}
	for _, r := range out.Rows {
		cells := map[string]accounting.PeriodHarness{}
		for _, h := range r.Harnesses {
			cells[h.Harness] = h
		}
		line := []string{r.Period}
		for _, name := range out.Harnesses {
			c := cells[name]
			line = append(line, fmt.Sprint(c.Commands), fmt.Sprint(c.Compacted), fmt.Sprint(c.CompactionTokens), fmt.Sprint(c.RoutingBelowTokens))
		}
		table = append(table, line)
	}
	widths := make([]int, len(header))
	for _, row := range table {
		for i, c := range row {
			if len(c) > widths[i] {
				widths[i] = len(c)
			}
		}
	}
	for _, row := range table {
		var b strings.Builder
		for i, c := range row {
			if i == 0 {
				fmt.Fprintf(&b, "%-*s", widths[i], c)
			} else {
				fmt.Fprintf(&b, "  %*s", widths[i], c)
			}
		}
		fmt.Fprintln(w, b.String())
	}
	fmt.Fprintln(w, "runs and compacted = Compactor runs; saved = tokens the Compactor kept out of the context; below = billable tokens that ran below the session model. Never added.")
}
