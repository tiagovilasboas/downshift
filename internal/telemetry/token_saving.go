// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package telemetry

import "fmt"

// Tokens saved from the requested model: the live, price-free view of what
// Downshift did, for every harness.
//
// A routed child runs about the same tokens on another model, so nothing here
// says the token total went down. A child's tokens are "saved from the
// requested model" when they ran on a cheaper model than the one the harness
// asked for (the #251 "cheaper" bucket, tokenDirection). The share is counted
// apart for work tokens (input and output, where models differ most in price)
// and cache tokens (read and write, where the gap is small or zero), so a
// cache-heavy child cannot make a small saving look large. Dollars are never
// computed here: `downshift report` prices tokens at report time.

// TokenSaving is one population of children: a child, an agent type or a
// session. Counts only: no prompt, task, path, session or agent id.
type TokenSaving struct {
	// Children counts the children with a priced token record.
	Children int `json:"children"`
	// SavedChildren counts those whose tokens ran on a cheaper model.
	SavedChildren int `json:"saved_children"`
	// Work is input plus output tokens; Cache is cache read plus write.
	WorkTokens  int64 `json:"work_tokens"`
	WorkSaved   int64 `json:"work_saved"`
	CacheTokens int64 `json:"cache_tokens"`
	CacheSaved  int64 `json:"cache_saved"`
	// WorkPricier and CachePricier ran on a pricier model than requested (an
	// upshift, or a tier move to a model that costs more): never a saving.
	WorkPricier  int64 `json:"work_pricier,omitempty"`
	CachePricier int64 `json:"cache_pricier,omitempty"`
	// SavedInferred counts saved children whose requested model was inferred
	// from settings, not sent by the harness: the saving rests on a guess.
	SavedInferred int `json:"saved_inferred,omitempty"`
	// AtFloor counts unchanged children whose requested model already sat on
	// the lowest catalog tier: no cheaper model existed, so a zero is not a miss.
	AtFloor int `json:"at_floor,omitempty"`
	// GatewayUnattributed counts children that ran on an OpenRouter id no
	// allowlisted row names: never saved, never guessed from a tier.
	// The JSON key is kept: it is a published field and only its wording changed.
	GatewayUnattributed int `json:"gateway_unpriced,omitempty"`
}

// Observed reports whether any child's tokens are known.
func (s TokenSaving) Observed() bool { return s.Children > 0 }

// WorkPercent is the share of work tokens saved from the requested model, in
// percent; 0 with no work tokens.
func (s TokenSaving) WorkPercent() float64 { return share(s.WorkSaved, s.WorkTokens) }

// CachePercent is the same share for cache tokens.
func (s TokenSaving) CachePercent() float64 { return share(s.CacheSaved, s.CacheTokens) }

// Direction is the bucket of a single child: cheaper, pricier or unchanged.
func (s TokenSaving) Direction() string {
	switch {
	case s.WorkSaved+s.CacheSaved > 0:
		return TokenCheaper
	case s.WorkPricier+s.CachePricier > 0:
		return TokenPricier
	}
	return TokenUnchanged
}

// Line is the short text every live view uses, or "" when nothing is observed:
// "saved 62% of work · 58% of cache". A zero says why when it can: every child
// already ran on the cheapest model, so there was nothing to save. The token
// counts stay in the JSON fields.
func (s TokenSaving) Line() string {
	if !s.Observed() {
		return ""
	}
	var out string
	switch {
	case s.WorkSaved+s.CacheSaved > 0:
		out = fmt.Sprintf("saved %s of work · %s of cache", percentText(s.WorkPercent()), percentText(s.CachePercent()))
	case s.AtFloor == s.Children:
		out = "already on the cheapest model"
	default:
		out = "nothing saved yet"
	}
	if p := s.WorkPricier + s.CachePricier; p > 0 {
		out += " · " + formatTokens(p) + " on a pricier model"
	}
	if s.SavedInferred > 0 {
		out += " · baseline inferred"
	}
	if s.GatewayUnattributed > 0 {
		out += " · " + gatewayUnattributedText(s.GatewayUnattributed)
	}
	return out
}

// gatewayUnattributedText counts gateway runs whose id no allowlisted row names.
func gatewayUnattributedText(n int) string {
	if n == 1 {
		return "1 gateway run unattributed: no tier for this gateway id"
	}
	return fmt.Sprintf("%d gateway runs unattributed: no tier for this gateway id", n)
}

func (s *TokenSaving) add(o TokenSaving) {
	s.Children += o.Children
	s.SavedChildren += o.SavedChildren
	s.WorkTokens += o.WorkTokens
	s.WorkSaved += o.WorkSaved
	s.CacheTokens += o.CacheTokens
	s.CacheSaved += o.CacheSaved
	s.WorkPricier += o.WorkPricier
	s.CachePricier += o.CachePricier
	s.SavedInferred += o.SavedInferred
	s.AtFloor += o.AtFloor
	s.GatewayUnattributed += o.GatewayUnattributed
}

// TokenSavingView is TokenSavings folded three ways.
type TokenSavingView struct {
	Total TokenSaving `json:"total"`
	// ByAgentType is keyed by the decision's agent type label; a decision
	// with no type (most harnesses send none) is under UntypedAgent.
	ByAgentType map[string]TokenSaving `json:"by_agent_type,omitempty"`
	// byChild is keyed by harness + "\n" + decision key.
	byChild map[string]TokenSaving
}

// UntypedAgent labels children whose spawn named no agent type.
const UntypedAgent = "untyped"

// Child returns the saving of the child spawned by decision d.
func (v TokenSavingView) Child(d Event) TokenSaving {
	return v.byChild[d.Harness+"\n"+DecisionKey(d)]
}

// TokenSavings folds the usage records of events into tokens saved from the
// requested model. The population and the buckets are those of
// Stats.TokensRerouted (priced usage records with tokens; control, held,
// refuted and unlinked children count in the totals and never as saved), so
// the live views and `stats` cannot disagree.
func TokenSavings(events []Event) TokenSavingView {
	a := newAggregator()
	a.indexDecisions(events)
	// The aggregator's index leaves out control decisions (outcome baseline);
	// a control child still belongs to its decision and its agent type.
	decisions := map[linkKey]Event{}
	for _, ev := range events {
		if IsDecisionRow(ev) {
			decisions[linkKey{ev.Harness, DecisionKey(ev)}] = ev
		}
	}
	type child struct {
		s        TokenSaving
		typ      string
		inferred bool
		floor    bool
		gateway  bool
	}
	children := map[string]*child{}
	for _, ev := range events {
		if ev.Outcome != OutcomeUsage || !ev.HasTokenUsage() {
			continue
		}
		c := classesOf(ev.TokenUsageOrZero())
		if c.Total == 0 {
			continue
		}
		key := linkKey{ev.Harness, ev.LinkedDecision}
		decision, found := decisions[key]
		work, cache := c.Input+c.Output, c.Cache()
		s := TokenSaving{WorkTokens: work, CacheTokens: cache}
		dir := tokenDirection(ev, decision, found, a.unrouted[key], a.refuted[key])
		requested, ran := tokenModels(ev, decision, found, a.refuted[key])
		switch dir {
		case TokenCheaper:
			s.WorkSaved, s.CacheSaved = work, cache
		case TokenPricier:
			s.WorkPricier, s.CachePricier = work, cache
		}
		// A resumed child has several records; they fold into one child.
		id := ev.Harness + "\n" + ev.LinkedDecision
		if !found {
			id = ev.Harness + "\nusage:" + DecisionKey(ev)
		}
		ch := children[id]
		if ch == nil {
			ch = &child{typ: UntypedAgent}
			if found && decision.AgentType != "" {
				ch.typ = decision.AgentType
			}
			children[id] = ch
		}
		ch.s.add(s)
		ch.inferred = ch.inferred || ev.BaselineSource == BaselineInferred || hasReason(decision.ReasonCodes, ReasonBaselineFromSettings)
		ch.floor = ch.floor || dir == TokenUnchanged && atTierFloor(ev.Harness, requested)
		ch.gateway = ch.gateway || isUnattributedGateway(dir, ran, found)
	}
	v := TokenSavingView{ByAgentType: map[string]TokenSaving{}, byChild: map[string]TokenSaving{}}
	for id, ch := range children {
		ch.s.Children = 1
		if ch.gateway {
			ch.s.GatewayUnattributed = 1
		}
		if ch.s.Direction() == TokenCheaper {
			ch.s.SavedChildren = 1
			if ch.inferred {
				ch.s.SavedInferred = 1
			}
		} else if ch.floor && ch.s.Direction() == TokenUnchanged {
			ch.s.AtFloor = 1
		}
		v.byChild[id] = ch.s
		v.Total.add(ch.s)
		t := v.ByAgentType[ch.typ]
		t.add(ch.s)
		v.ByAgentType[ch.typ] = t
	}
	return v
}

func share(part, whole int64) float64 {
	if whole <= 0 {
		return 0
	}
	return 100 * float64(part) / float64(whole)
}

func percentText(v float64) string { return fmt.Sprintf("%.0f%%", v) }
