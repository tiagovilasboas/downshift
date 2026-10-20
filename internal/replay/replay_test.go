// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package replay

import (
	"context"
	"testing"
)

type fixedSource struct{ c Corpus }

func (f fixedSource) Cases(context.Context) (Corpus, error) { return f.c, nil }

// fakeCmp prices by a fixed rank: a lower rank is cheaper.
type fakeCmp map[string]int

func (f fakeCmp) Known(m string) bool { _, ok := f[m]; return ok }
func (f fakeCmp) Compare(from, to string) Direction {
	a, okA := f[from]
	b, okB := f[to]
	switch {
	case !okA || !okB:
		return Unknown
	case a == b:
		return Same
	case b < a:
		return Cheaper
	default:
		return Pricier
	}
}

// ruleRouter moves every case to the model named in its policy knob.
type ruleRouter struct{ to string }

func (r ruleRouter) Route(c Case, p Policy) (Verdict, error) {
	if p.Knobs["move"] == "1" {
		return Verdict{Model: r.to}, nil
	}
	return Verdict{Held: true, Reasons: []string{"no_rewrite"}}, nil
}

func TestEvaluateCountsTokensMovedAndAgreement(t *testing.T) {
	cmp := fakeCmp{"big": 3, "mid": 2, "small": 1}
	cases := []Case{
		{ID: "a", Prompt: "p", Baseline: "big", Ran: "big", Tokens: Tokens{Input: 10, Output: 5}},
		{ID: "b", Prompt: "p", Baseline: "big", Ran: "big", Tokens: Tokens{CacheRead: 100}},
	}
	rep, err := Evaluate(context.Background(), fixedSource{Corpus{Cases: cases, Skipped: map[SkipReason]int{}}},
		ruleRouter{to: "small"}, cmp, []Policy{
			{Name: "hold"},
			{Name: "move", Knobs: map[string]string{"move": "1"}},
		})
	if err != nil {
		t.Fatal(err)
	}
	if rep.Evaluated != 2 || rep.Actual.Agree != 2 {
		t.Fatalf("evaluated %d, actual agree %d; want 2 and 2", rep.Evaluated, rep.Actual.Agree)
	}
	hold, move := rep.Policies[0], rep.Policies[1]
	if hold.Held != 2 || hold.Agree != 2 || hold.Cheaper.Spawns != 0 {
		t.Errorf("hold: held %d agree %d cheaper %d; want 2, 2, 0", hold.Held, hold.Agree, hold.Cheaper.Spawns)
	}
	if move.Cheaper.Spawns != 2 || move.Cheaper.Tokens.Total() != 115 || move.Agree != 0 {
		t.Errorf("move: cheaper %d spawns %d tokens, agree %d; want 2, 115, 0", move.Cheaper.Spawns, move.Cheaper.Tokens.Total(), move.Agree)
	}
	if move.MoreSavings != 2 {
		t.Errorf("move: more savings %d, want 2 (policy cheaper than what ran)", move.MoreSavings)
	}
}

func TestAliasesAreOneModel(t *testing.T) {
	// "sonnet" and the full id are the same model: a hold must still agree.
	cmp := fakeCmp{"sonnet": 2, "claude-sonnet-5-5": 2}
	cases := []Case{{ID: "a", Prompt: "p", Baseline: "sonnet", Ran: "claude-sonnet-5-5", Tokens: Tokens{Input: 1}}}
	rep, _ := Evaluate(context.Background(), fixedSource{Corpus{Cases: cases, Skipped: map[SkipReason]int{}}},
		ruleRouter{}, cmp, []Policy{{Name: "hold"}})
	if rep.Policies[0].Agree != 1 {
		t.Fatalf("alias hold agree = %d, want 1", rep.Policies[0].Agree)
	}
}

func TestUnpricedCasesAreSkippedBeforeAnyPolicy(t *testing.T) {
	cmp := fakeCmp{"big": 3}
	cases := []Case{
		{ID: "a", Prompt: "p", Baseline: "big", Ran: "mystery", Tokens: Tokens{Input: 1}},
		{ID: "b", Prompt: "", Baseline: "big", Ran: "big", Tokens: Tokens{Input: 1}},
		{ID: "c", Prompt: "p", Baseline: "big", Ran: "big"},
	}
	rep, _ := Evaluate(context.Background(), fixedSource{Corpus{Cases: cases, Skipped: map[SkipReason]int{}}},
		ruleRouter{}, cmp, []Policy{{Name: "hold"}})
	if rep.Evaluated != 0 {
		t.Fatalf("evaluated %d, want 0", rep.Evaluated)
	}
	if rep.Skipped[SkipUnpricedModel] != 1 || rep.Skipped[SkipNoPrompt] != 1 || rep.Skipped[SkipNoUsage] != 1 {
		t.Errorf("skips = %v", rep.Skipped)
	}
}
