// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/tiagovilasboas/downshift/internal/accounting"
	"github.com/tiagovilasboas/downshift/internal/quota"
	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

// quotaWindowRoles maps a native window id to the report column it fills. The
// ids are the ones internal/quota writes for each harness (Claude Code
// statusline, Codex usage report).
var quotaWindowRoles = map[string]string{
	"five_hour":       "5h",
	"seven_day":       "week",
	"codex/primary":   "5h",
	"codex/secondary": "week",
}

// quotaUsedFor reads the native snapshot of a harness. No snapshot (or one
// that cannot be read) is QuotaUnknown. Only harness-wide windows count.
func quotaUsedFor(harness string, now time.Time) accounting.QuotaUsed {
	s, _ := quota.Load(harness)
	if s == nil || s.Validate() != nil {
		return accounting.QuotaUsed{Status: accounting.QuotaUnknown}
	}
	q := accounting.QuotaUsed{Status: accounting.QuotaNative, ObservedAt: s.ObservedAt, Stale: now.After(s.ExpiresAt)}
	for _, w := range s.Windows {
		if w.Scope != "harness" || w.UsedPercent == nil {
			continue
		}
		pct := *w.UsedPercent
		switch quotaWindowRoles[w.ID] {
		case "5h":
			q.FiveHourPct = &pct
		case "week":
			q.WeekPct = &pct
		}
	}
	return q
}

// quotaByHarness reads the snapshot of every known harness.
func quotaByHarness(now time.Time) map[string]accounting.QuotaUsed {
	out := map[string]accounting.QuotaUsed{}
	for _, h := range knownHarnessNames() {
		out[h] = quotaUsedFor(h, now)
	}
	return out
}

// printTokensByHarness writes one line per harness with two token columns that
// are never summed, then the quota used. A harness with no usage bound to a
// decision shows "unobserved" in the moved column, never 0.
func printTokensByHarness(w io.Writer, rows []accounting.HarnessTokens, scope accounting.Scope) {
	fmt.Fprintf(w, "\nTOKENS PER HARNESS (%s; the two token columns are never added)\n", scope.Label())
	if len(rows) == 0 {
		fmt.Fprintln(w, "   No events in this window.")
		fmt.Fprintf(w, "   no events: %s\n", strings.Join(knownHarnessNames(), ", "))
		return
	}
	fmt.Fprintf(w, "   %-14s %-26s %-32s %s\n", "harness", "tokens avoided (Compactor)", "tokens moved to a smaller model", "quota used (5h · week)")
	have := map[string]bool{}
	for _, r := range rows {
		have[r.Harness] = true
		moved := "unobserved"
		if r.MovedObserved {
			moved = telemetry.ShortTokens(r.MovedToSmaller)
		}
		fmt.Fprintf(w, "   %-14s %-26s %-32s %s\n", r.Harness, telemetry.ShortTokens(r.CompactorAvoided), moved, quotaText(r.Quota))
	}
	if rest := harnessesWithoutEvents(have); len(rest) > 0 {
		fmt.Fprintf(w, "   no events: %s\n", strings.Join(rest, ", "))
	}
}

// quotaText renders "42% · 17%", "42% · unknown", or "unknown". A snapshot
// past its expiry says so.
func quotaText(q accounting.QuotaUsed) string {
	if q.Status != accounting.QuotaNative || (q.FiveHourPct == nil && q.WeekPct == nil) {
		return "unknown"
	}
	pct := func(p *float64) string {
		if p == nil {
			return "unknown"
		}
		return fmt.Sprintf("%.0f%%", *p)
	}
	out := pct(q.FiveHourPct) + " · " + pct(q.WeekPct)
	if q.Stale {
		out += " (snapshot expired)"
	}
	return out
}
