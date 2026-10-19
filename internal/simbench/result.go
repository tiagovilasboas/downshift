// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package simbench

import (
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/tiagovilasboas/downshift/internal/core"
	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

// EstimateLabel heads every simulation output. Simulated dollars are list
// prices applied to synthetic tokens: rungs A and B of docs/guides/evidence.md.
const EstimateLabel = "ESTIMATE (evidence rungs A/B): catalog list prices on synthetic tokens, no model call; no dollar percentage. " +
	"Not a measurement: quality and real token counts need real runs or `downshift report`."

// Money is one dollar comparison over routed (non-control) spawns.
type Money struct {
	Events       int
	BaselineUSD  float64
	ActualUSD    float64
	GrossUSD     float64
	OverspendUSD float64
}

// NetUSD is gross saving minus overspend; negative means routing cost more.
func (m Money) NetUSD() float64 { return m.GrossUSD - m.OverspendUSD }

// Tokens is a token count by class. Routing moves tokens to another model; it
// does not remove them.
type Tokens struct {
	Input, Output, CacheRead, CacheWrite int64
}

func (t Tokens) Total() int64 { return t.Input + t.Output + t.CacheRead + t.CacheWrite }

func (t *Tokens) add(u telemetry.TokenUsage) {
	t.Input += u.InputTokens
	t.Output += u.OutputTokens
	t.CacheRead += u.CachedTokens - u.CacheWriteTokens
	t.CacheWrite += u.CacheWriteTokens
}

// HarnessResult is one harness's run of a scenario.
type HarnessResult struct {
	Harness string
	Spawns  int
	Control int
	Routed  int
	// Rewrites: routed spawns the hook wrote a model for. Downshifts and
	// Upshifts split them by price; Holds are spawns a guardrail held; Kept
	// are routed spawns left alone without a hold.
	Rewrites, Downshifts, Upshifts, Holds, Kept int
	Honored, Refuted                            int
	UnknownBaseline                             int
	// UsageSeam: the harness has a proven child-token source, so the
	// production pipeline prices it (rung B). Otherwise only rung A exists.
	UsageSeam bool
	// Pipeline is what the production aggregation credits (rung B).
	Pipeline Money
	// WhatIf prices the same synthetic tokens on the requested and the ran
	// model for every routed spawn the catalog prices (rung A).
	WhatIf Money
	// MovedCheaper and MovedPricier are the routed tokens that ran on a
	// cheaper or pricier model than requested; Unchanged ran as requested.
	MovedCheaper, MovedPricier, Unchanged Tokens
	ReasonCodes                           map[string]int
}

// Result is one scenario run.
type Result struct {
	Scenario   Scenario
	Harnesses  []HarnessResult
	Violations []Violation
	Traces     []Trace
	Events     []telemetry.Event
}

func summarize(sc Scenario, traces []Trace, events []telemetry.Event, cat core.Resolver) []HarnessResult {
	var out []HarnessResult
	for _, h := range sc.harnessList() {
		v := viewOf(h, events)
		caps, _ := core.CapsFor(h)
		r := HarnessResult{Harness: h, ReasonCodes: map[string]int{},
			UsageSeam: caps.UsageSource != "" && caps.UsageSource != core.UsageNone}
		for _, t := range traces {
			if t.Harness != h {
				continue
			}
			r.Spawns++
			if t.Control {
				r.Control++
				continue
			}
			r.Routed++
			dec := v.decisions[t.Key()]
			key := telemetry.DecisionKey(dec)
			for _, c := range dec.ReasonCodes {
				r.ReasonCodes[c]++
			}
			if dec.FromModel == "unknown" {
				r.UnknownBaseline++
			}
			switch {
			case t.Written != "":
				r.Rewrites++
				switch p, q := price(cat, h, t.Written), price(cat, h, t.Requested); {
				case p < q:
					r.Downshifts++
				case p > q:
					r.Upshifts++
				}
				for _, res := range v.resolved[key] {
					if res.RewriteHonored != nil && *res.RewriteHonored {
						r.Honored++
					} else if res.RewriteHonored != nil {
						r.Refuted++
					}
				}
			case len(dec.Corrections) > 0:
				r.Holds++
			default:
				r.Kept++
			}
			r.whatIf(cat, t)
			// Rung B: the production records as the aggregation credits them.
			for _, u := range v.usage[key] {
				if !u.HasRealCost() || v.refuted(key) {
					continue
				}
				c := telemetry.RoutedCost(u, dec, true)
				r.Pipeline.Events++
				r.Pipeline.BaselineUSD += *c.BaselineCostUSD
				r.Pipeline.ActualUSD += *c.ActualCostUSD
				if d := c.RealDeltaUSD(); d > 0 {
					r.Pipeline.GrossUSD += d
				} else {
					r.Pipeline.OverspendUSD -= d
				}
			}
		}
		out = append(out, r)
	}
	return out
}

// whatIf adds one routed spawn to rung A and to the token split.
func (r *HarnessResult) whatIf(cat core.Resolver, t Trace) {
	req, okReq := cat.LookupByID(t.Harness, t.Requested)
	ran, okRan := cat.LookupByID(t.Harness, t.Ran)
	base, actual := 0.0, 0.0
	if okReq && okRan && telemetry.HasModelPrices(req) && telemetry.HasModelPrices(ran) {
		base, actual = telemetry.CostUSD(req, t.Usage), telemetry.CostUSD(ran, t.Usage)
		r.WhatIf.Events++
		r.WhatIf.BaselineUSD += base
		r.WhatIf.ActualUSD += actual
		if base > actual {
			r.WhatIf.GrossUSD += base - actual
		} else {
			r.WhatIf.OverspendUSD += actual - base
		}
	}
	switch p, q := price(cat, t.Harness, t.Ran), price(cat, t.Harness, t.Requested); {
	case t.Ran != t.Requested && p < q:
		r.MovedCheaper.add(t.Usage)
	case t.Ran != t.Requested && p > q:
		r.MovedPricier.add(t.Usage)
	default:
		r.Unchanged.add(t.Usage)
	}
}

// Format writes the scenario's per-harness table under the estimate label.
func (r Result) Format(w io.Writer) {
	fmt.Fprintf(w, "%s\n", EstimateLabel)
	fmt.Fprintf(w, "scenario %s (seed %d, parent tier %s, agent %s)\n", r.Scenario.Name, r.Scenario.Seed, r.Scenario.ParentTier, orDefault(r.Scenario.AgentType))
	fmt.Fprintf(w, "%-12s %6s %4s %6s %5s %4s %3s %5s %4s %5s %5s  %-24s %-24s %s\n",
		"harness", "spawns", "ctl", "routed", "rewr", "down", "up", "holds", "kept", "honor", "unkBL",
		"rung B (pipeline)", "rung A (what-if)", "tokens saved work / cache (routed)")
	for _, h := range r.Harnesses {
		pipeline := "unobserved (no usage source)"
		if h.UsageSeam {
			pipeline = money(h.Pipeline)
		}
		fmt.Fprintf(w, "%-12s %6d %4d %6d %5d %4d %3d %5d %4d %5d %5d  %-24s %-24s %s\n",
			h.Harness, h.Spawns, h.Control, h.Routed, h.Rewrites, h.Downshifts, h.Upshifts, h.Holds, h.Kept,
			h.Honored, h.UnknownBaseline, pipeline, money(h.WhatIf), h.savedShare())
	}
	if len(r.Violations) == 0 {
		fmt.Fprintln(w, "invariants: all held")
		return
	}
	fmt.Fprintf(w, "invariants: %d violated\n", len(r.Violations))
	for _, v := range r.Violations {
		fmt.Fprintf(w, "  %s\n", v)
	}
}

// savedShare is the routed tokens that ran on a cheaper model than requested,
// work (input+output) and cache apart, from the synthetic tokens.
func (h HarnessResult) savedShare() string {
	all := Tokens{}
	for _, t := range []Tokens{h.MovedCheaper, h.MovedPricier, h.Unchanged} {
		all.Input, all.Output, all.CacheRead, all.CacheWrite = all.Input+t.Input, all.Output+t.Output, all.CacheRead+t.CacheRead, all.CacheWrite+t.CacheWrite
	}
	c := h.MovedCheaper
	work := shareOf(c.Input+c.Output, all.Input+all.Output)
	cache := shareOf(c.CacheRead+c.CacheWrite, all.CacheRead+all.CacheWrite)
	out := fmt.Sprintf("%.0f%% / %.0f%% of %s", work, cache, human(all.Total()))
	if p := h.MovedPricier.Total(); p > 0 {
		out += " · " + human(p) + " pricier"
	}
	return out
}

func shareOf(part, whole int64) float64 {
	if whole <= 0 {
		return 0
	}
	return 100 * float64(part) / float64(whole)
}

// TopReasons lists a harness's most frequent reason codes, for a test log.
func (h HarnessResult) TopReasons(n int) string {
	type kv struct {
		code string
		n    int
	}
	var rows []kv
	for c, k := range h.ReasonCodes {
		rows = append(rows, kv{c, k})
	}
	slices.SortFunc(rows, func(a, b kv) int {
		if a.n != b.n {
			return b.n - a.n
		}
		return strings.Compare(a.code, b.code)
	})
	var parts []string
	for i, r := range rows {
		if i == n {
			break
		}
		parts = append(parts, fmt.Sprintf("%s %d", r.code, r.n))
	}
	return strings.Join(parts, ", ")
}

func money(m Money) string {
	if m.Events == 0 {
		return "unpriced"
	}
	return fmt.Sprintf("$%.4f net", m.NetUSD())
}

func human(n int64) string {
	switch {
	case n >= 1_000_000:
		return fmt.Sprintf("%.1fM", float64(n)/1e6)
	case n >= 1_000:
		return fmt.Sprintf("%.1fk", float64(n)/1e3)
	}
	return fmt.Sprintf("%d", n)
}

func orDefault(agentType string) string {
	if agentType == "" {
		return "general-purpose"
	}
	return agentType
}
