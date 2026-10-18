// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package telemetry

import (
	"fmt"
	"io"
	"sort"
	"strings"
)

// CurationRow aggregates routing decisions of one harness at one tier and
// effort, with the provider-reported usage linked to them. It exists so an
// owner can review what the router chose and what it cost, per harness. It
// holds counts and token totals only: no prompts, no task text, no paths.
type CurationRow struct {
	Harness string
	Tier    string
	Effort  string

	Spawns    int // decision events
	Applied   int // a rewrite was written
	Held      int // allow or blocked: the spawn ran unchanged
	Upshift   int
	Downshift int

	HeldBy map[string]int // guardrail rule id -> count

	Usage     int // linked usage records
	InputTok  int64
	OutputTok int64
	CachedTok int64
	CostUSD   float64 // catalog-priced estimate from usage, not a bill
}

func isDecision(ev Event) bool {
	return ev.Outcome == OutcomeRewriteEmitted || ev.Outcome == OutcomeAllow || ev.Outcome == "blocked"
}

type curationKey struct{ harness, tier, effort string }

func norm(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "" {
		return "unknown"
	}
	return s
}

// BuildCuration groups decisions by harness, tier and final effort and adds
// the usage records that link to them. Usage with no resolvable decision is
// reported under tier "(unlinked)" so no token is silently dropped. A usage
// record seen twice for the same agent counts once.
func BuildCuration(events []Event) []CurationRow {
	rows := map[curationKey]*CurationRow{}
	row := func(k curationKey) *CurationRow {
		if r, ok := rows[k]; ok {
			return r
		}
		r := &CurationRow{Harness: k.harness, Tier: k.tier, Effort: k.effort, HeldBy: map[string]int{}}
		rows[k] = r
		return r
	}
	byDecision := map[string]curationKey{} // harness|decision key -> row key
	for _, ev := range events {
		if !isDecision(ev) {
			continue
		}
		k := curationKey{norm(ev.Harness), norm(ev.Tier), norm(ev.FinalEffort)}
		byDecision[k.harness+"|"+DecisionKey(ev)] = k
		r := row(k)
		r.Spawns++
		if AppliedRewrite(ev.Outcome, ev.Corrections) && ev.Outcome == OutcomeRewriteEmitted {
			r.Applied++
		} else {
			r.Held++
			for _, c := range ev.Corrections {
				r.HeldBy[c]++
			}
		}
		switch ev.Verdict {
		case "UPSHIFT":
			r.Upshift++
		case "DOWNSHIFT":
			r.Downshift++
		}
	}
	seen := map[string]bool{}
	for _, ev := range events {
		if ev.Outcome != OutcomeUsage {
			continue
		}
		if ev.AgentHash != "" {
			id := norm(ev.Harness) + "|" + ev.AgentHash
			if seen[id] {
				continue
			}
			seen[id] = true
		}
		k, ok := byDecision[norm(ev.Harness)+"|"+ev.LinkedDecision]
		if !ok {
			k = curationKey{norm(ev.Harness), "(unlinked)", "-"}
		}
		r := row(k)
		r.Usage++
		if ev.InputTokens != nil {
			r.InputTok += *ev.InputTokens
		}
		if ev.OutputTokens != nil {
			r.OutputTok += *ev.OutputTokens
		}
		if ev.CachedTokens != nil {
			r.CachedTok += *ev.CachedTokens
		}
		if ev.ActualCostUSD != nil {
			r.CostUSD += *ev.ActualCostUSD
		}
	}
	out := make([]CurationRow, 0, len(rows))
	for _, r := range rows {
		out = append(out, *r)
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.Harness != b.Harness {
			return a.Harness < b.Harness
		}
		if a.Tier != b.Tier {
			return a.Tier < b.Tier
		}
		return a.Effort < b.Effort
	})
	return out
}

// PrintCuration writes the review table as Markdown, so it can be pasted into
// a note or an issue. Cost is a catalog-priced estimate from recorded usage.
func PrintCuration(w io.Writer, rows []CurationRow) {
	fmt.Fprintln(w, "# Spawns by harness, tier and effort")
	fmt.Fprintln(w)
	if len(rows) == 0 {
		fmt.Fprintln(w, "No decisions recorded yet.")
		return
	}
	fmt.Fprintln(w, "| Harness | Tier | Effort | Spawns | Applied | Held | Up | Down | Usage | In tok | Out tok | Cached tok | Est. USD |")
	fmt.Fprintln(w, "|---|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|")
	held := map[string]map[string]int{}
	for _, r := range rows {
		fmt.Fprintf(w, "| %s | %s | %s | %d | %d | %d | %d | %d | %d | %d | %d | %d | %.4f |\n",
			r.Harness, r.Tier, r.Effort, r.Spawns, r.Applied, r.Held, r.Upshift, r.Downshift,
			r.Usage, r.InputTok, r.OutputTok, r.CachedTok, r.CostUSD)
		for rule, n := range r.HeldBy {
			if held[r.Harness] == nil {
				held[r.Harness] = map[string]int{}
			}
			held[r.Harness][rule] += n
		}
	}
	if len(held) > 0 {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "## Held by guardrail")
		var hs []string
		for h := range held {
			hs = append(hs, h)
		}
		sort.Strings(hs)
		for _, h := range hs {
			var rs []string
			for rule := range held[h] {
				rs = append(rs, rule)
			}
			sort.Strings(rs)
			for _, rule := range rs {
				fmt.Fprintf(w, "- %s: %s x%d\n", h, rule, held[h][rule])
			}
		}
	}
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Held = the spawn ran unchanged (allow, blocked or a guardrail). Tier is the recommendation;")
	fmt.Fprintln(w, "effort is applied only where the harness can (Codex today). Usage rows with tier (unlinked)")
	fmt.Fprintln(w, "could not be tied to a decision. Cost is a catalog-priced estimate, not a bill.")
}
