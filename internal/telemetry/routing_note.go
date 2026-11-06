// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package telemetry

import "regexp"

// identPattern is the identifier shape routing/contract and the policy loader
// enforce on rule ids and profile names. It is repeated here because telemetry
// must not import the router (core).
var identPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_.-]{0,63}$`)

// RoutingNote is the routing note of one decision as the event log holds it:
// enums, tiers and declared rule or profile ids only (routing/contract.Note
// without its headroom fields). It says why a spawn changed or did not. No
// task text, path or prompt can be in it: Clean drops any value that is not an
// identifier. A decision written before the note, or by a path that does not
// record it, has none.
type RoutingNote struct {
	Verdict  string `json:"verdict"`
	Reason   string `json:"reason,omitempty"`
	RuleID   string `json:"rule_id,omitempty"`
	Profile  string `json:"profile,omitempty"`
	TierFrom string `json:"tier_from,omitempty"`
	TierTo   string `json:"tier_to,omitempty"`
}

// Clean keeps only the fields that are well-formed identifiers, the shape the
// policy loader enforces on rule ids and profile names. A note whose verdict is
// not an identifier comes back empty (Verdict ""): the reader counts it as not
// recorded instead of showing free text.
func (n RoutingNote) Clean() RoutingNote {
	ident := func(s string) string {
		if identPattern.MatchString(s) {
			return s
		}
		return ""
	}
	return RoutingNote{
		Verdict: ident(n.Verdict), Reason: ident(n.Reason), RuleID: ident(n.RuleID),
		Profile: ident(n.Profile), TierFrom: ident(n.TierFrom), TierTo: ident(n.TierTo),
	}
}
