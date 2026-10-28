// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package decide

import (
	"testing"

	"github.com/tiagovilasboas/downshift/internal/core"
	"github.com/tiagovilasboas/downshift/internal/routing/contract"
	"github.com/tiagovilasboas/downshift/internal/routing/policy"
)

// TestDecideDefaultProfiles runs the default profile table (precedence 5)
// through the full decision, guard included, on the harness it is keyed by.
func TestDecideDefaultProfiles(t *testing.T) {
	const cc contract.HarnessID = "claude-code"
	ccReq := func(agentType, requested string) contract.Request {
		return contract.Request{Harness: cc, RequestedModel: requested, AgentType: agentType}
	}
	declared := &policy.Policy{
		Profiles: []policy.Profile{{Name: "Explore", Tier: contract.TierMid, Harness: cc}},
	}
	cases := []struct {
		name    string
		request contract.Request
		pol     *policy.Policy
		risk    []string
		// class and confident are the classifier result the class step reads.
		class     string
		confident bool
		want      want
	}{
		{name: "Explore on frontier goes small",
			request: ccReq("Explore", "frontier-model"),
			want:    want{contract.VerdictDefault, "", "small-model", "", "", contract.TierSmall, core.VerdictDownshift}},
		{name: "claude-code-guide on mid goes small",
			request: ccReq("claude-code-guide", "mid-model"),
			want:    want{contract.VerdictDefault, "", "small-model", "", "", contract.TierSmall, core.VerdictDownshift}},
		{name: "statusline-setup on mid goes small",
			request: ccReq("statusline-setup", "mid-model"),
			want:    want{contract.VerdictDefault, "", "small-model", "", "", contract.TierSmall, core.VerdictDownshift}},
		{name: "type match ignores case",
			request: ccReq("explore", "frontier-model"),
			want:    want{contract.VerdictDefault, "", "small-model", "", "", contract.TierSmall, core.VerdictDownshift}},
		{name: "already small has nothing to move",
			request: ccReq("Explore", "small-model"),
			want:    want{contract.VerdictKept, "", "small-model", "", "", contract.TierSmall, core.VerdictOK}},
		{name: "general-purpose keeps the requested model",
			request: ccReq("general-purpose", "frontier-model"),
			want:    want{contract.VerdictKept, "", "frontier-model", "", "", contract.TierFrontier, core.VerdictOK}},
		{name: "general-purpose on a confident routine task goes mid",
			request: ccReq("general-purpose", "frontier-model"), class: "TRIVIAL", confident: true,
			want: want{contract.VerdictDefault, "", "mid-model", "", "", contract.TierMid, core.VerdictDownshift}},
		{name: "general-purpose on a confident MEDIUM task goes mid",
			request: ccReq("general-purpose", "frontier-model"), class: "MEDIUM", confident: true,
			want: want{contract.VerdictDefault, "", "mid-model", "", "", contract.TierMid, core.VerdictDownshift}},
		{name: "general-purpose keeps when the classifier is unsure",
			request: ccReq("general-purpose", "frontier-model"), class: "TRIVIAL",
			want: want{contract.VerdictKept, "", "frontier-model", "", "", contract.TierFrontier, core.VerdictOK}},
		{name: "general-purpose keeps a confident COMPLEX task",
			request: ccReq("general-purpose", "frontier-model"), class: "COMPLEX", confident: true,
			want: want{contract.VerdictKept, "", "frontier-model", "", "", contract.TierFrontier, core.VerdictOK}},
		{name: "general-purpose keeps a routine task that touches a risk",
			request: ccReq("general-purpose", "frontier-model"), class: "TRIVIAL", confident: true, risk: []string{"secret"},
			want: want{contract.VerdictKept, "", "frontier-model", "", "", contract.TierFrontier, core.VerdictOK}},
		{name: "general-purpose never goes small from the class step",
			request: ccReq("general-purpose", "mid-model"), class: "TRIVIAL", confident: true,
			want: want{contract.VerdictKept, "", "mid-model", "", "", contract.TierMid, core.VerdictOK}},
		{name: "Plan keeps a confident routine task",
			request: ccReq("Plan", "frontier-model"), class: "TRIVIAL", confident: true,
			want: want{contract.VerdictKept, "", "frontier-model", "", "", contract.TierFrontier, core.VerdictOK}},
		{name: "Plan keeps the requested model",
			request: ccReq("Plan", "frontier-model"),
			want:    want{contract.VerdictKept, "", "frontier-model", "", "", contract.TierFrontier, core.VerdictOK}},
		{name: "type without a row keeps",
			request: ccReq("mechanical-worker", "frontier-model"),
			want:    want{contract.VerdictKept, "", "frontier-model", "", "", contract.TierFrontier, core.VerdictOK}},
		// Risk asks for mid, and the unchanged guard keeps the requested tier.
		{name: "risk on frontier is held by the guard",
			request: ccReq("Explore", "frontier-model"), risk: []string{"secret"},
			want: want{contract.VerdictGuardFloor, contract.ReasonRiskFloor, "frontier-model", "", "", contract.TierFrontier, core.VerdictOK}},
		{name: "risk never goes small from mid",
			request: ccReq("Explore", "mid-model"), risk: []string{"auth"},
			want: want{contract.VerdictKept, "", "mid-model", "", "", contract.TierMid, core.VerdictOK}},
		// Anything declared wins over the table.
		{name: "declared agent-type profile beats the default",
			request: ccReq("Explore", "frontier-model"), pol: declared,
			want: want{contract.VerdictProfile, "", "mid-model", "", "Explore", contract.TierMid, core.VerdictDownshift}},
		{name: "hint beats the default",
			request: func() contract.Request {
				r := ccReq("Explore", "frontier-model")
				r.Hint.Tier = contract.TierMid
				return r
			}(),
			want: want{contract.VerdictHintDown, "", "mid-model", "", "", contract.TierMid, core.VerdictDownshift}},
		{name: "pinned model beats the default",
			request: func() contract.Request {
				r := ccReq("Explore", "frontier-model")
				r.Pinned = true
				return r
			}(),
			want: want{contract.VerdictKept, "", "frontier-model", "", "", contract.TierFrontier, core.VerdictOK}},
		{name: "the table is keyed by harness",
			request: contract.Request{Harness: h, RequestedModel: "frontier-model", AgentType: "Explore"},
			want:    want{contract.VerdictKept, "", "frontier-model", "", "", contract.TierFrontier, core.VerdictOK}},
		// A declared profile of the same name, refused for another harness, is
		// not replaced by the table: the spawn keeps its model.
		{name: "declared profile of another harness is not replaced by the table",
			request: ccReq("Explore", "frontier-model"),
			pol: &policy.Policy{Profiles: []policy.Profile{
				{Name: "Explore", Tier: contract.TierSmall, Harness: "other"}}},
			want: want{contract.VerdictKept, "", "frontier-model", "", "", contract.TierFrontier, core.VerdictOK}},
		// A hint tier equal to the request is a declared word: the table keeps out.
		{name: "equal hint tier blocks the default table",
			request: func() contract.Request {
				r := ccReq("Explore", "frontier-model")
				r.Hint.Tier = contract.TierFrontier
				return r
			}(),
			want: want{contract.VerdictKept, "", "frontier-model", "", "", contract.TierFrontier, core.VerdictOK}},
		// The table never widens: a risky Explore on a small model is kept as
		// kept, not recorded as a refused upshift.
		{name: "risky default on a small model is kept, not an upshift refusal",
			request: ccReq("Explore", "small-model"), risk: []string{"secret"},
			want: want{contract.VerdictKept, "", "small-model", "", "", contract.TierSmall, core.VerdictOK}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			d, n := Decide(Input{Request: c.request, Policy: c.pol, Now: now, Risk: c.risk, Catalog: newCatalog(), Class: c.class, Confident: c.confident})
			got := want{n.Verdict, n.Reason, d.Model.ID, n.RuleID, n.Profile, n.TierTo, d.Verdict}
			if got != c.want {
				t.Errorf("got %+v\nwant %+v", got, c.want)
			}
		})
	}
}
