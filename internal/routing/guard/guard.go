// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

// Package guard decides whether one spawn may move down to a smaller tier. It
// can only refuse a downshift; it never grants a model the caller did not
// already decide on, and it never changes anything but the answer.
//
// Rules (docs/design/routing-v2.md section 2, guard):
//   - never widen: a target above the requested tier is refused;
//   - explicit_only: a model marked routing:"explicit_only" is never chosen
//     automatically;
//   - foreign id: a target the harness catalog does not know is refused;
//   - doubt: an unconfident decision, or an unknown tier, keeps the requested
//     model.
//
// The catalog is an interface because routing packages may import only core,
// paths and other routing packages (internal/archtest/routing_test.go):
// catalog.Catalog satisfies it, and routing never imports catalog.
//
// Downshift by Tiago Vilas Boas
// https://github.com/tiagovilasboas/downshift
package guard

import (
	"github.com/tiagovilasboas/downshift/internal/core"
	"github.com/tiagovilasboas/downshift/internal/routing/contract"
)

// Catalog is the part of the harness model catalog the guard reads.
type Catalog interface {
	// IsExactID reports an exact id or declared alias for the harness. A
	// family-prefix match is not exact, so a foreign version string is refused.
	core.ExactIDResolver
	// IsExplicitOnly reports routing:"explicit_only" for the harness's model.
	IsExplicitOnly(harness, modelID string) bool
}

// Input is one proposed move of one spawn. It carries ids, tiers and category
// names only, never task text.
type Input struct {
	Harness   contract.HarnessID
	Confident bool          // false is doubt: the requested model is kept
	Requested contract.Tier // tier of the model the spawn asked for
	Target    contract.Tier // tier the decision proposes
	TargetID  string        // model id the target tier resolves to, verbatim
}

// Result is the guard's answer. Allowed is true only for a downshift that
// passed every check; the caller then runs TargetID. Otherwise the requested
// model is kept, and Reason names the refusing check. Reason is empty for
// doubt and for a target that is not below the requested tier.
type Result struct {
	Allowed bool
	Reason  contract.Reason
}

// Check applies the guard to one proposed move. The checks run in the order
// below and the first that refuses decides the reason:
//  1. doubt: no catalog, an unconfident decision or an unknown tier keeps the
//     requested model;
//  2. never widen: a target above the requested tier is refused; an equal tier
//     has nothing to move;
//  3. explicit_only: the target model is marked explicit_only;
//  4. foreign id: the target id is not known to the harness catalog.
//
// The built-in risk veto is gone (owner decision O8): the orchestrator chooses
// the model, and a project that wants a floor declares it in policy.json.
func Check(cat Catalog, in Input) Result {
	from, to := rank(in.Requested), rank(in.Target)
	if cat == nil || !in.Confident || from < 0 || to < 0 {
		return Result{}
	}
	switch {
	case to > from:
		return refuse(contract.ReasonNeverWiden)
	case to == from:
		return Result{}
	}
	harness := string(in.Harness)
	if cat.IsExplicitOnly(harness, in.TargetID) {
		return refuse(contract.ReasonExplicitOnly)
	}
	if !cat.IsExactID(harness, in.TargetID) {
		return refuse(contract.ReasonForeignID)
	}
	return Result{Allowed: true}
}

func refuse(reason contract.Reason) Result {
	return Result{Reason: reason}
}

// rank orders tiers from cheapest (0) to strongest, following contract.Tiers.
// An unknown tier ranks -1.
func rank(t contract.Tier) int {
	for i, s := range contract.Tiers() {
		if s == t {
			return i
		}
	}
	return -1
}
