// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

// Package replay is the offline counterfactual layer: it takes spawns that
// really happened (a prompt, the model the child would have run on, the model
// it did run on, the tokens it used), asks the router what a policy would have
// chosen for the same prompt, and counts the tokens each policy moves to a
// cheaper or a pricier model. It makes no model call and no network
// connection, and it never persists a prompt.
//
// It re-decides real spawns to measure a policy, through its own ports.
//
// The domain imports nothing from the router, the catalog, telemetry or any
// harness. Everything outside it comes through three ports: Source (where the
// cases come from), Router (how a policy decides) and Comparer (which of two
// models is cheaper). Adapters implement them: replay/ccsource reads Claude
// Code transcripts, replay/corerouter calls core.Route.
//
// What it proves: how many tokens a policy would move, and how often it agrees
// with what really ran. What it does not prove: quality. The tokens of a case
// are the tokens the child really used; a cheaper model may use more.
//
// Downshift by Tiago Vilas Boas · https://github.com/tiagovilasboas/downshift
package replay

import (
	"context"
	"fmt"
	"sort"
)

// Tokens is one child's usage by class, as the harness reported it.
type Tokens struct {
	Input      int64
	Output     int64
	CacheRead  int64
	CacheWrite int64
}

// Work is input plus output: the tokens that are not a cache class.
func (t Tokens) Work() int64 { return t.Input + t.Output }

// Cache is the cache read plus the cache write.
func (t Tokens) Cache() int64 { return t.CacheRead + t.CacheWrite }

// Total is every class.
func (t Tokens) Total() int64 { return t.Work() + t.Cache() }

func (t Tokens) add(o Tokens) Tokens {
	return Tokens{t.Input + o.Input, t.Output + o.Output, t.CacheRead + o.CacheRead, t.CacheWrite + o.CacheWrite}
}

// Case is one real spawn. Prompt lives in memory for the router and is never
// copied into a Report, a log or a file.
type Case struct {
	// ID is an opaque stable key (a hash); it never carries task text.
	ID        string
	AgentType string
	Prompt    string
	// Baseline is the model the child would have run on without a rewrite. A
	// Source emits a case only when it is certain of it.
	Baseline string
	// Ran is the model the child really ran on.
	Ran    string
	Tokens Tokens
}

// SkipReason says why a Source or Evaluate left a spawn out. A skipped spawn
// is counted, never guessed: an unknown baseline would turn into a fake saving.
type SkipReason string

const (
	SkipNoPrompt        SkipReason = "no_prompt"
	SkipNoUsage         SkipReason = "no_usage"
	SkipNoBaseline      SkipReason = "baseline_unknown"
	SkipNoRan           SkipReason = "ran_model_unknown"
	SkipPluginAgent     SkipReason = "plugin_agent_type"
	SkipModelDiffers    SkipReason = "model_differs_from_baseline"
	SkipUnpricedModel   SkipReason = "model_not_in_catalog"
	SkipChildNotStopped SkipReason = "child_not_finished"
)

// Corpus is what a Source reads: the usable cases and a count of what it left
// out and why.
type Corpus struct {
	Cases   []Case
	Skipped map[SkipReason]int
}

// Source reads real spawns.
type Source interface {
	Cases(ctx context.Context) (Corpus, error)
}

// Direction compares two models by price.
type Direction int

const (
	Unknown Direction = iota
	Same
	Cheaper
	Pricier
)

func (d Direction) String() string {
	switch d {
	case Same:
		return "same"
	case Cheaper:
		return "cheaper"
	case Pricier:
		return "pricier"
	default:
		return "unknown"
	}
}

// Comparer tells whether to is cheaper than from. Known is false for a model id
// the catalog cannot resolve.
type Comparer interface {
	Known(model string) bool
	Compare(from, to string) Direction
}

// Policy is a named router configuration. Knobs are opaque to the domain; the
// Router adapter interprets them.
type Policy struct {
	Name  string
	Knobs map[string]string
}

// Verdict is what a Router chose for one case.
type Verdict struct {
	// Model is the model the policy would write for the child, "" when it
	// leaves the spawn alone.
	Model string
	// Held is true when a guardrail or doubt kept the requested model.
	Held    bool
	Reasons []string
}

// Router decides one case under a policy.
type Router interface {
	Route(c Case, p Policy) (Verdict, error)
}

// Bucket is the tokens that moved in one direction.
type Bucket struct {
	Spawns int
	Tokens Tokens
}

func (b *Bucket) add(c Case) {
	b.Spawns++
	b.Tokens = b.Tokens.add(c.Tokens)
}

// PolicyResult is one policy over the whole corpus.
type PolicyResult struct {
	Policy                 Policy
	Cheaper, Pricier, Same Bucket
	// Agree counts cases where the policy chose the model that really ran.
	Agree int
	// Held counts cases the policy kept on the requested model.
	Held int
	// MoreSavings and LessSavings count cases where the policy moved the child
	// to a cheaper (more) or a pricier (less) model than what really ran.
	MoreSavings, LessSavings int
	// Reasons counts the router's reason codes over held cases.
	Reasons map[string]int
}

// Report is the outcome of Evaluate. It holds counts only.
type Report struct {
	// Evaluated is the number of cases every policy decided.
	Evaluated int
	Skipped   map[SkipReason]int
	// Actual is what really happened: baseline against the model that ran.
	Actual   PolicyResult
	Policies []PolicyResult
}

// Evaluate replays the corpus under each policy. A case whose models the
// Comparer cannot price is skipped before any policy sees it, so every policy
// is measured on the same cases.
func Evaluate(ctx context.Context, src Source, router Router, cmp Comparer, policies []Policy) (Report, error) {
	corpus, err := src.Cases(ctx)
	if err != nil {
		return Report{}, fmt.Errorf("read cases: %w", err)
	}
	rep := Report{Skipped: map[SkipReason]int{}}
	for r, n := range corpus.Skipped {
		rep.Skipped[r] += n
	}
	var usable []Case
	for _, c := range corpus.Cases {
		switch {
		case c.Prompt == "":
			rep.Skipped[SkipNoPrompt]++
		case c.Tokens.Total() == 0:
			rep.Skipped[SkipNoUsage]++
		case c.Baseline == "":
			rep.Skipped[SkipNoBaseline]++
		case c.Ran == "":
			rep.Skipped[SkipNoRan]++
		case !cmp.Known(c.Baseline) || !cmp.Known(c.Ran):
			rep.Skipped[SkipUnpricedModel]++
		default:
			usable = append(usable, c)
		}
	}
	rep.Evaluated = len(usable)
	rep.Actual = PolicyResult{Policy: Policy{Name: "actual"}, Reasons: map[string]int{}}
	for _, c := range usable {
		tally(&rep.Actual, cmp.Compare(c.Baseline, c.Ran), c)
		rep.Actual.Agree++
	}
	for _, p := range policies {
		res := PolicyResult{Policy: p, Reasons: map[string]int{}}
		for _, c := range usable {
			if err := ctx.Err(); err != nil {
				return Report{}, err
			}
			v, err := router.Route(c, p)
			if err != nil {
				return Report{}, fmt.Errorf("policy %s: %w", p.Name, err)
			}
			chosen := c.Baseline
			if v.Model != "" && !v.Held {
				chosen = v.Model
			}
			if v.Held {
				res.Held++
				for _, r := range v.Reasons {
					res.Reasons[r]++
				}
			}
			tally(&res, cmp.Compare(c.Baseline, chosen), c)
			if cmp.Compare(chosen, c.Ran) == Same {
				res.Agree++
				continue
			}
			switch cmp.Compare(c.Ran, chosen) {
			case Cheaper:
				res.MoreSavings++
			case Pricier:
				res.LessSavings++
			}
		}
		rep.Policies = append(rep.Policies, res)
	}
	return rep, nil
}

func tally(r *PolicyResult, d Direction, c Case) {
	switch d {
	case Cheaper:
		r.Cheaper.add(c)
	case Pricier:
		r.Pricier.add(c)
	default:
		r.Same.add(c)
	}
}

// SkipCount is one skip reason with its count.
type SkipCount struct {
	Reason SkipReason
	Count  int
}

// SkipLines returns the skip counts in a stable order for printing.
func (r Report) SkipLines() []SkipCount {
	out := make([]SkipCount, 0, len(r.Skipped))
	for reason, n := range r.Skipped {
		out = append(out, SkipCount{reason, n})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Reason < out[j].Reason })
	return out
}
