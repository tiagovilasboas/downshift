// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package simbench

import (
	"bytes"
	"fmt"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/tiagovilasboas/downshift/internal/core"
	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

// Invariant rule ids. Each is one accounting promise the event log must keep.
const (
	RuleExitZero        = "hook-exit-zero"
	RuleOneDecision     = "one-decision-per-spawn"
	RuleWireLog         = "wire-matches-log"
	RuleControl         = "control-group"
	RuleControlLinked   = "control-linked-to-end"
	RuleOneUsage        = "one-usage-per-child"
	RuleBaselineTruth   = "baseline-is-requested-model"
	RuleSavedArithmetic = "saved-equals-baseline-minus-actual"
	RuleSummary         = "summary-totals"
	RuleRates           = "control-excluded-from-rates"
	RuleHonorOnce       = "honored-counted-once"
	RuleUnknownBaseline = "no-saving-on-unknown-baseline"
	RulePercent         = "no-impossible-percentage"
	RuleUnapplied       = "no-saving-without-rewrite"
	RulePromptFree      = "prompt-free"
	RuleTokens          = "tokens-match-children"
	RuleExpect          = "scenario-expectation"
)

// Violation is one broken promise, with enough detail to find the event.
type Violation struct {
	Rule    string
	Harness string
	Detail  string
}

func (v Violation) String() string { return v.Rule + " [" + v.Harness + "]: " + v.Detail }

// eps absorbs float summation order; every figure is a sum of a few dozen
// list-price products.
const eps = 1e-9

// harnessView is the event log of one harness, indexed for the checks.
type harnessView struct {
	events    []telemetry.Event
	decisions map[string]telemetry.Event // by hashed tool-use id
	usage     map[string][]telemetry.Event
	resolved  map[string][]telemetry.Event
	ended     map[string][]telemetry.Event
	unlinked  int // usage records with no linked decision
	strays    []telemetry.Event
}

func viewOf(harness string, events []telemetry.Event) harnessView {
	v := harnessView{
		decisions: map[string]telemetry.Event{},
		usage:     map[string][]telemetry.Event{},
		resolved:  map[string][]telemetry.Event{},
		ended:     map[string][]telemetry.Event{},
	}
	for _, ev := range events {
		if ev.Harness != harness {
			continue
		}
		v.events = append(v.events, ev)
		switch {
		case ev.Outcome == telemetry.OutcomeUsage:
			if ev.LinkedDecision == "" {
				v.unlinked++
			}
			v.usage[ev.LinkedDecision] = append(v.usage[ev.LinkedDecision], ev)
		case ev.Outcome == telemetry.OutcomeResolved:
			v.resolved[ev.LinkedDecision] = append(v.resolved[ev.LinkedDecision], ev)
		case ev.Outcome == telemetry.OutcomeChildEnded:
			v.ended[ev.LinkedDecision] = append(v.ended[ev.LinkedDecision], ev)
		case telemetry.IsObservationOutcome(ev.Outcome) || telemetry.IsFailureOutcome(ev.Outcome):
			v.strays = append(v.strays, ev)
		default:
			if _, dup := v.decisions[ev.ToolUseHash]; dup || ev.ToolUseHash == "" {
				v.strays = append(v.strays, ev)
				continue
			}
			v.decisions[ev.ToolUseHash] = ev
		}
	}
	return v
}

// refuted reports a linked resolved record saying the harness did not run the
// rewrite.
func (v harnessView) refuted(key string) bool {
	for _, r := range v.resolved[key] {
		if r.RewriteHonored != nil && !*r.RewriteHonored {
			return true
		}
	}
	return false
}

// applied is a decision whose rewrite the hook wrote and no guardrail held.
func applied(dec telemetry.Event) bool {
	return dec.Outcome == telemetry.OutcomeRewriteEmitted && telemetry.AppliedRewrite(dec.Outcome, dec.Corrections)
}

// Check holds the event log to the traces. It is pure: the same traces and
// events always give the same violations, so a test can feed it a log with a
// planted defect.
func Check(sc Scenario, traces []Trace, events []telemetry.Event, cat core.Resolver) []Violation {
	var out []Violation
	byHarness := map[string][]Trace{}
	var order []string
	for _, t := range traces {
		if _, ok := byHarness[t.Harness]; !ok {
			order = append(order, t.Harness)
		}
		byHarness[t.Harness] = append(byHarness[t.Harness], t)
	}
	for _, h := range order {
		out = append(out, checkHarness(sc, h, byHarness[h], viewOf(h, events), cat)...)
	}
	return out
}

func checkHarness(sc Scenario, h string, traces []Trace, v harnessView, cat core.Resolver) []Violation {
	var out []Violation
	bad := func(rule, format string, args ...any) {
		out = append(out, Violation{Rule: rule, Harness: h, Detail: fmt.Sprintf(format, args...)})
	}
	caps, _ := core.CapsFor(h)
	usageSeam := caps.UsageSource != "" && caps.UsageSource != core.UsageNone
	w := wires[h]
	lookup := func(id string) (core.Model, bool) {
		if id == "" || id == "unknown" {
			return core.Model{}, false
		}
		m, ok := cat.LookupByID(h, id)
		return m, ok && telemetry.HasModelPrices(m)
	}

	if len(v.strays) > 0 {
		bad(RuleOneDecision, "%d duplicate decisions or failure records", len(v.strays))
	}
	if len(v.decisions) != len(traces) {
		bad(RuleOneDecision, "%d decisions for %d spawns", len(v.decisions), len(traces))
	}

	var routed, control, pricedRouted, pricedControl int
	var wantGross, wantOver float64
	var wantDownshifted, wantHonored int
	// Token oracle: each priced child's tokens, in the bucket the definition
	// in telemetry/tokens_rerouted.go puts them, and per report arm.
	wantTokens := map[string]*Tokens{}
	var wantRoutedTokens, wantControlTokens Tokens
	addTokens := func(dir string, u telemetry.TokenUsage) {
		if wantTokens[dir] == nil {
			wantTokens[dir] = &Tokens{}
		}
		wantTokens[dir].add(u)
	}
	for _, t := range traces {
		if t.RC != 0 {
			bad(RuleExitZero, "spawn %s: hook exit %d", short(t.Key()), t.RC)
		}
		dec, ok := v.decisions[t.Key()]
		if !ok {
			bad(RuleOneDecision, "spawn %s: no decision recorded", short(t.Key()))
			continue
		}
		key := telemetry.DecisionKey(dec)
		isControl := slices.Contains(dec.ReasonCodes, telemetry.ReasonControl)

		// Control group: the production draw, never rewritten, a baseline event.
		switch {
		case t.Control && (!isControl || dec.Outcome != telemetry.OutcomeBaseline || t.Written != ""):
			bad(RuleControl, "spawn %s: drawn for control but outcome %s, CONTROL %v, written %q", short(t.Key()), dec.Outcome, isControl, t.Written)
		case !t.Control && (isControl || dec.Outcome == telemetry.OutcomeBaseline):
			bad(RuleControl, "spawn %s: routed spawn recorded as control (outcome %s)", short(t.Key()), dec.Outcome)
		}
		if t.Control {
			control++
		} else {
			routed++
		}

		// What the wire said and what the log says agree.
		if t.Written != "" {
			// A harness with strict model names gets an alias on the wire
			// (Claude Code: "haiku"); the log keeps the catalog id.
			if dec.Outcome != telemetry.OutcomeRewriteEmitted || !sameModel(cat, h, dec.ToModel, t.Written) {
				bad(RuleWireLog, "spawn %s: wire wrote %q, log has outcome %s final %q", short(t.Key()), t.Written, dec.Outcome, dec.ToModel)
			}
		} else if dec.Outcome == telemetry.OutcomeRewriteEmitted || dec.ToModel != dec.FromModel {
			bad(RuleWireLog, "spawn %s: wire left it unchanged, log has outcome %s %s -> %s", short(t.Key()), dec.Outcome, dec.FromModel, dec.ToModel)
		}
		if dec.EstimatedSavings < 0 || dec.EstimatedSavings >= 1 {
			bad(RulePercent, "spawn %s: estimated_savings %v outside [0,1)", short(t.Key()), dec.EstimatedSavings)
		}

		// Every child end links to its decision, control spawns included.
		if n := len(v.ended[key]); n != 1 {
			rule := RuleOneUsage
			if t.Control {
				rule = RuleControlLinked
			}
			bad(rule, "spawn %s: %d child_ended records linked, want 1", short(t.Key()), n)
		}

		// The baseline the log prices is the model the child would have run on.
		if dec.FromModel != "unknown" {
			if got, ok1 := cat.LookupByID(h, dec.FromModel); ok1 {
				if want, ok2 := cat.LookupByID(h, t.Requested); !ok2 || want.ID != got.ID {
					bad(RuleBaselineTruth, "spawn %s: log baseline %s, child would run %s", short(t.Key()), dec.FromModel, t.Requested)
				}
			}
		} else if !sc.ExpectFor(h).UnknownBaseline && sc.AgentType != AgentUndefined {
			bad(RuleBaselineTruth, "spawn %s: baseline unknown for a type whose model is known", short(t.Key()))
		}

		isApplied := applied(dec)
		isRefuted := v.refuted(key)
		honoredRecords := 0
		for _, r := range v.resolved[key] {
			if r.RewriteHonored != nil {
				honoredRecords++
				if !telemetry.IsAppliedShift(dec) {
					bad(RuleHonorOnce, "spawn %s: honor verdict on a decision that changed nothing", short(t.Key()))
				}
			}
		}
		if telemetry.IsAppliedShift(dec) && w.reportsRan {
			if honoredRecords != 1 {
				bad(RuleHonorOnce, "spawn %s: %d honor verdicts, want 1", short(t.Key()), honoredRecords)
			}
			if isRefuted != sc.Refuse {
				bad(RuleHonorOnce, "spawn %s: refuted %v, harness refused %v", short(t.Key()), isRefuted, sc.Refuse)
			}
			if !isRefuted {
				wantHonored++
			}
		}
		if isApplied && !isRefuted && dec.Verdict == "DOWNSHIFT" {
			wantDownshifted++
		}

		// Usage: exactly one priced record where the harness has a proven
		// token source, none elsewhere.
		usage := v.usage[key]
		if !usageSeam {
			for _, u := range usage {
				if u.ActualCostUSD != nil || u.BaselineCostUSD != nil {
					bad(RuleOneUsage, "spawn %s: priced usage on a harness with no proven usage source", short(t.Key()))
				}
			}
			continue
		}
		if len(usage) != 1 {
			bad(RuleOneUsage, "spawn %s: %d usage records linked, want 1", short(t.Key()), len(usage))
			continue
		}
		u := usage[0]
		reqM, okReq := lookup(t.Requested)
		ranM, okRan := lookup(t.Ran)
		if okRan {
			if t.Control {
				pricedControl++
				wantControlTokens.add(t.Usage)
			} else {
				pricedRouted++
				wantRoutedTokens.add(t.Usage)
			}
			switch {
			case t.Control || !isApplied || isRefuted:
				addTokens(telemetry.TokenUnchanged, t.Usage)
			case dec.FromModel == "unknown" || !okReq:
				addTokens(telemetry.TokenUnattributed, t.Usage)
			default:
				switch d := telemetry.CostUSD(reqM, t.Usage) - telemetry.CostUSD(ranM, t.Usage); {
				case sc.Refuse || d == 0:
					addTokens(telemetry.TokenUnchanged, t.Usage)
				case d > 0:
					addTokens(telemetry.TokenCheaper, t.Usage)
				default:
					addTokens(telemetry.TokenPricier, t.Usage)
				}
			}
			if u.ActualCostUSD == nil || math.Abs(*u.ActualCostUSD-telemetry.CostUSD(ranM, t.Usage)) > eps {
				bad(RuleSavedArithmetic, "spawn %s: actual cost %v, want %v", short(t.Key()), ptr(u.ActualCostUSD), telemetry.CostUSD(ranM, t.Usage))
			}
		}
		if dec.FromModel == "unknown" {
			if u.BaselineCostUSD != nil {
				bad(RuleUnknownBaseline, "spawn %s: baseline cost %v on an unknown requested model", short(t.Key()), *u.BaselineCostUSD)
			}
			continue
		}
		if !okReq || !okRan {
			continue
		}
		if u.BaselineCostUSD == nil || math.Abs(*u.BaselineCostUSD-telemetry.CostUSD(reqM, t.Usage)) > eps {
			bad(RuleSavedArithmetic, "spawn %s: baseline cost %v, want %v", short(t.Key()), ptr(u.BaselineCostUSD), telemetry.CostUSD(reqM, t.Usage))
		}
		credited := 0.0
		if !isRefuted {
			credited = telemetry.RoutedCost(u, dec, true).RealDeltaUSD()
		}
		want := 0.0
		if isApplied && !sc.Refuse {
			want = telemetry.CostUSD(reqM, t.Usage) - telemetry.CostUSD(ranM, t.Usage)
		}
		if math.Abs(credited-want) > eps {
			bad(RuleSavedArithmetic, "spawn %s: credited saving %v, want %v (applied %v, control %v)", short(t.Key()), credited, want, isApplied, t.Control)
		}
		if t.Control && credited != 0 {
			bad(RuleRates, "spawn %s: control spawn credited %v", short(t.Key()), credited)
		}
		if base := telemetry.CostUSD(reqM, t.Usage); credited > 0 && base > 0 {
			if frac, bound := credited/base, maxSavingFraction(reqM, ranM, t.Usage); frac > bound+eps || frac > 1 {
				bad(RulePercent, "spawn %s: saving %.1f%% above the list-price gap %.1f%%", short(t.Key()), 100*frac, 100*bound)
			}
		}
		wantGross += max(want, 0)
		wantOver += max(-want, 0)
	}

	// Production aggregation, run on this harness's log.
	sum := telemetry.Summarize(v.events)
	if math.Abs(sum.GrossSavedUSD-wantGross) > eps || math.Abs(sum.OverspendUSD-wantOver) > eps ||
		math.Abs(sum.NetSavedUSD-(sum.GrossSavedUSD-sum.OverspendUSD)) > eps {
		bad(RuleSummary, "summary gross %v overspend %v net %v, want gross %v overspend %v", sum.GrossSavedUSD, sum.OverspendUSD, sum.NetSavedUSD, wantGross, wantOver)
	}
	if sum.OverspendUSD < 0 || sum.GrossSavedUSD < 0 {
		bad(RuleSummary, "negative gross %v or overspend %v", sum.GrossSavedUSD, sum.OverspendUSD)
	}
	if v.unlinked > 0 {
		bad(RuleOneUsage, "%d usage records linked to no decision", v.unlinked)
	}
	st := telemetry.Aggregate(v.events)
	if st.Total != routed || st.Baseline != control {
		bad(RuleRates, "stats total %d baseline %d, want routed %d control %d", st.Total, st.Baseline, routed, control)
	}
	if st.Downshifted != wantDownshifted {
		bad(RuleUnapplied, "stats downshifted %d, want %d applied and not refuted", st.Downshifted, wantDownshifted)
	}
	if w.reportsRan && st.HonorObserved != wantHonored {
		bad(RuleHonorOnce, "stats honor observed %d, want %d", st.HonorObserved, wantHonored)
	}
	out = append(out, checkTokens(h, usageSeam, st, wantTokens)...)
	out = append(out, checkTokensSaved(h, sc, v.events, wantTokens)...)
	rep := telemetry.BuildReport(v.events, 0)
	if rep.Routed.Spawns != routed || rep.Control.Spawns != control || rep.Excluded != 0 {
		bad(RuleRates, "report routed %d control %d excluded %d, want %d / %d / 0", rep.Routed.Spawns, rep.Control.Spawns, rep.Excluded, routed, control)
	}
	if rep.Routed.Ended+rep.Control.Ended != len(traces) {
		bad(RuleControlLinked, "report ended %d+%d of %d spawns", rep.Routed.Ended, rep.Control.Ended, len(traces))
	}
	if usageSeam && (rep.Routed.Completed != pricedRouted || rep.Control.Completed != pricedControl) {
		bad(RuleRates, "report completed routed %d control %d, want %d / %d", rep.Routed.Completed, rep.Control.Completed, pricedRouted, pricedControl)
	}
	if usageSeam && (!sameTokens(rep.Routed.Tokens, wantRoutedTokens) || !sameTokens(rep.Control.Tokens, wantControlTokens)) {
		bad(RuleTokens, "report tokens routed %+v control %+v, want %+v / %+v", rep.Routed.Tokens, rep.Control.Tokens, wantRoutedTokens, wantControlTokens)
	}
	if rep.Routed.Completed > 0 && math.Abs(rep.Routed.TokensPerTask-float64(rep.Routed.Tokens.Total)/float64(rep.Routed.Completed)) > eps {
		bad(RuleTokens, "routed tokens per task %v is not tokens over completed tasks", rep.Routed.TokensPerTask)
	}
	if !usageSeam && (rep.Routed.Completed != 0 || rep.Control.Completed != 0) {
		bad(RuleRates, "report priced tasks on a harness with no usage source")
	}

	// A log holding only the spawns the router did not change must show no
	// saving of any kind: the "quoted a saving the hook cannot deliver" class.
	unapplied := map[string]bool{}
	for key, dec := range v.decisions {
		if !applied(dec) || v.refuted(key) {
			unapplied[key] = true
		}
	}
	var subset []telemetry.Event
	for _, ev := range v.events {
		k := ev.LinkedDecision
		if !telemetry.IsObservationOutcome(ev.Outcome) {
			k = telemetry.DecisionKey(ev)
		}
		if unapplied[k] {
			subset = append(subset, ev)
		}
	}
	if s := telemetry.Aggregate(subset); s.RealSavedUSD != 0 || s.RealOverspendUSD != 0 || s.NormRouted != s.NormBaseline || s.Downshifted+s.Upshifted != 0 {
		bad(RuleUnapplied, "unchanged spawns credited: saved %v, overspend %v, units %v/%v, shifted %d",
			s.RealSavedUSD, s.RealOverspendUSD, s.NormRouted, s.NormBaseline, s.Downshifted+s.Upshifted)
	}

	out = append(out, checkExpect(sc, h, traces, v, cat)...)
	return out
}

// checkTokens holds the production token buckets to the children's tokens:
// routing moves tokens between models, so every priced child's tokens appear
// in exactly one bucket, and a harness with no usage source says unobserved.
func checkTokens(h string, usageSeam bool, st telemetry.Stats, want map[string]*Tokens) []Violation {
	var out []Violation
	bad := func(format string, args ...any) {
		out = append(out, Violation{Rule: RuleTokens, Harness: h, Detail: fmt.Sprintf(format, args...)})
	}
	got, ok := st.TokensRerouted.ByHarness[h]
	if !ok {
		bad("no token entry for the harness")
		return out
	}
	if !usageSeam {
		if got.Status != telemetry.TokenStatusUnobserved || got.Cheaper != nil || got.Pricier != nil {
			bad("status %q on a harness with no usage source, want %q and no buckets", got.Status, telemetry.TokenStatusUnobserved)
		}
		return out
	}
	for _, b := range []struct {
		dir string
		got *telemetry.TokenClasses
	}{
		{telemetry.TokenCheaper, got.Cheaper}, {telemetry.TokenPricier, got.Pricier},
		{telemetry.TokenUnchanged, got.Unchanged}, {telemetry.TokenUnattributed, got.Unattributed},
	} {
		w := Tokens{}
		if want[b.dir] != nil {
			w = *want[b.dir]
		}
		g := telemetry.TokenClasses{}
		if b.got != nil {
			g = *b.got
		}
		if !sameTokens(g, w) {
			bad("%s tokens %+v, want %+v", b.dir, g, w)
		}
	}
	return out
}

// checkTokensSaved holds the live tokens-saved figures (session tree, panel,
// statusline, stats headline) to the children's tokens: work and cache saved
// are exactly the cheaper bucket, the totals are every priced child, and the
// one agent type of the scenario carries the same numbers.
func checkTokensSaved(h string, sc Scenario, events []telemetry.Event, want map[string]*Tokens) []Violation {
	var out []Violation
	bad := func(format string, args ...any) {
		out = append(out, Violation{Rule: RuleTokens, Harness: h, Detail: fmt.Sprintf(format, args...)})
	}
	var all, saved, pricier Tokens
	for dir, t := range want {
		all.Input, all.Output, all.CacheRead, all.CacheWrite = all.Input+t.Input, all.Output+t.Output, all.CacheRead+t.CacheRead, all.CacheWrite+t.CacheWrite
		switch dir {
		case telemetry.TokenCheaper:
			saved = *t
		case telemetry.TokenPricier:
			pricier = *t
		}
	}
	view := telemetry.TokenSavings(events)
	got := view.Total
	if got.WorkTokens != all.Input+all.Output || got.CacheTokens != all.CacheRead+all.CacheWrite ||
		got.WorkSaved != saved.Input+saved.Output || got.CacheSaved != saved.CacheRead+saved.CacheWrite ||
		got.WorkPricier != pricier.Input+pricier.Output || got.CachePricier != pricier.CacheRead+pricier.CacheWrite {
		bad("tokens saved %+v, want work %d of %d, cache %d of %d, pricier %d", got,
			saved.Input+saved.Output, all.Input+all.Output, saved.CacheRead+saved.CacheWrite, all.CacheRead+all.CacheWrite, pricier.Total())
	}
	if got.WorkPercent() > 100 || got.CachePercent() > 100 || got.WorkPercent() < 0 {
		bad("impossible tokens-saved share: work %.1f%% cache %.1f%%", got.WorkPercent(), got.CachePercent())
	}
	if len(view.ByAgentType) > 1 {
		bad("%d agent types in a one-type scenario", len(view.ByAgentType))
	}
	for typ, t := range view.ByAgentType {
		if t != got {
			bad("agent type %s %+v differs from the total %+v", typ, t, got)
		}
	}
	return out
}

func sameTokens(g telemetry.TokenClasses, w Tokens) bool {
	return g.Input == w.Input && g.Output == w.Output && g.CacheRead == w.CacheRead &&
		g.CacheWrite == w.CacheWrite && g.Total == w.Total()
}

// checkExpect holds the scenario's own expectations for one harness.
func checkExpect(sc Scenario, h string, traces []Trace, v harnessView, cat core.Resolver) []Violation {
	e := sc.ExpectFor(h)
	var out []Violation
	bad := func(format string, args ...any) {
		out = append(out, Violation{Rule: RuleExpect, Harness: h, Detail: fmt.Sprintf(format, args...)})
	}
	routed, written := 0, 0
	for _, t := range traces {
		if t.Control {
			continue
		}
		routed++
		if t.Written != "" {
			written++
		}
		if e.NoDownshift && t.Written != "" && price(cat, h, t.Ran) < price(cat, h, t.Requested) {
			bad("spawn %s moved to a cheaper model (%s -> %s) on a task that must not downshift", short(t.Key()), t.Requested, t.Ran)
		}
		if dec, ok := v.decisions[t.Key()]; ok && e.UnknownBaseline && dec.FromModel != "unknown" {
			bad("spawn %s recorded baseline %s, want unknown", short(t.Key()), dec.FromModel)
		}
	}
	if routed == 0 {
		return out
	}
	share := float64(written) / float64(routed)
	if e.MinRewriteShare != nil && share < *e.MinRewriteShare {
		bad("rewrite share %.2f below %.2f (%d of %d routed)", share, *e.MinRewriteShare, written, routed)
	}
	if e.MaxRewriteShare != nil && share > *e.MaxRewriteShare {
		bad("rewrite share %.2f above %.2f (%d of %d routed)", share, *e.MaxRewriteShare, written, routed)
	}
	return out
}

// maxSavingFraction is the largest share of the baseline a move from req to
// ran can save on this usage: one minus the smallest per-class price ratio
// among the token classes present. Anything above it is arithmetic gone wrong.
func maxSavingFraction(req, ran core.Model, u telemetry.TokenUsage) float64 {
	reads := u.CachedTokens - u.CacheWriteTokens
	classes := []telemetry.TokenUsage{
		{InputTokens: u.InputTokens}, {OutputTokens: u.OutputTokens},
		{CachedTokens: reads}, {CachedTokens: u.CacheWriteTokens, CacheWriteTokens: u.CacheWriteTokens},
	}
	minRatio := math.Inf(1)
	for _, c := range classes {
		base := telemetry.CostUSD(req, c)
		if base <= 0 {
			continue
		}
		minRatio = min(minRatio, telemetry.CostUSD(ran, c)/base)
	}
	if math.IsInf(minRatio, 1) {
		return 0
	}
	return 1 - minRatio
}

func sameModel(cat core.Resolver, h, a, b string) bool {
	if a == b {
		return true
	}
	ma, okA := cat.LookupByID(h, a)
	mb, okB := cat.LookupByID(h, b)
	return okA && okB && ma.ID == mb.ID
}

func price(cat core.Resolver, h, id string) float64 {
	m, ok := cat.LookupByID(h, id)
	if !ok {
		return 0
	}
	return m.InputM + m.OutputM
}

// ScanPromptFree reads every file the hook path wrote under dirs and fails on
// any task text, planted canary, path from a task, or raw harness id. The
// fixtures the simulated harness wrote (transcripts, rollouts) are input and
// live elsewhere; agent definitions under .claude are skipped for the same
// reason.
func ScanPromptFree(traces []Trace, dirs ...string) ([]Violation, error) {
	needles := map[string]bool{Canary: true, sessionPrefix: true, toolPrefix: true, agentPrefix: true}
	for _, t := range traces {
		needles[t.Task] = true
		for _, word := range strings.Fields(t.Task) {
			if strings.Contains(word, "/") && len(word) >= 5 {
				needles[strings.Trim(word, ".,;:()`'\"")] = true
			}
		}
	}
	var out []Violation
	for _, root := range dirs {
		err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				if d.Name() == ".claude" {
					return filepath.SkipDir
				}
				return nil
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			for n := range needles {
				if n != "" && bytes.Contains(data, []byte(n)) {
					rel, _ := filepath.Rel(root, path)
					out = append(out, Violation{Rule: RulePromptFree, Harness: "*", Detail: fmt.Sprintf("%s holds %q", rel, clip(n))})
				}
			}
			return nil
		})
		if err != nil && !os.IsNotExist(err) {
			return nil, err
		}
	}
	slices.SortFunc(out, func(a, b Violation) int { return strings.Compare(a.Detail, b.Detail) })
	return out, nil
}

func short(id string) string {
	if len(id) > 8 {
		return id[:8]
	}
	return id
}

func clip(s string) string {
	if len(s) > 40 {
		return s[:40] + "…"
	}
	return s
}

func ptr(p *float64) any {
	if p == nil {
		return "nil"
	}
	return *p
}
