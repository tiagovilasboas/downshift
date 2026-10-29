// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package contract

// HarnessID names a harness ("claude-code", "kirocrew", ...), as the adapters
// declare it in core.HarnessCapabilities. It is an identifier, never text.
type HarnessID string

// Kind is the coarse kind of work a delegation declares. The set follows the
// REA section 2 (indirect routing inputs). Design section 5 listed another
// vocabulary; the contract follows the REA, so R4 and R10 use this set. An
// unknown value is ignored by the hint parser (design section 5), never an
// error. The zero value means no kind was declared.
type Kind string

const (
	KindNone     Kind = ""
	KindResearch Kind = "research"
	KindCode     Kind = "code"
	KindReview   Kind = "review"
	KindCommand  Kind = "command"
	KindScrap    Kind = "scrap"
	KindDocs     Kind = "docs"
)

// Kinds lists every declared Kind in a stable order.
func Kinds() []Kind {
	return []Kind{KindResearch, KindCode, KindReview, KindCommand, KindScrap, KindDocs}
}

// Valid reports whether k is one of Kinds.
func (k Kind) Valid() bool { return member(k, Kinds()) }

// EffortClass is the declared effort of a delegation (REA section 2; design
// section 5). The zero value means no effort class was declared; a rule with
// an effort list does not match it (REA section 2, stage 1).
type EffortClass string

const (
	EffortNone     EffortClass = ""
	EffortLight    EffortClass = "light"
	EffortStandard EffortClass = "standard"
	EffortDeep     EffortClass = "deep"
)

// EffortClasses lists every declared EffortClass in a stable order.
func EffortClasses() []EffortClass {
	return []EffortClass{EffortLight, EffortStandard, EffortDeep}
}

// Valid reports whether e is one of EffortClasses.
func (e EffortClass) Valid() bool { return member(e, EffortClasses()) }

// Hint is what the parent declares about a delegation: enums only (design
// section 2, precedence 4). A hint may move the tier down or keep it, never up
// past the requested model. The zero Hint declares nothing.
type Hint struct {
	Kind   Kind
	Effort EffortClass
	Tier   Tier // optional ceiling; TierUnknown when absent
}

// Request is the input of one routing decision: one spawn. It carries ids and
// enums from the payload and never the task text (AGENTS.md: task text stays
// in memory only; the guard's risk scan reads it in its own call, not from
// here). The decision is per spawn and stateless (design section 11): nothing
// here refers to a session list.
type Request struct {
	Harness HarnessID
	// RequestedModel is the model id the caller pinned or the harness default,
	// verbatim; empty when the payload names none. Doubt keeps it.
	RequestedModel string
	// Pinned is true when the caller set RequestedModel explicitly (an agent
	// definition model:, a --model flag) and false for a harness default.
	// A pinned model is kept unless the guard forbids it (precedence 1).
	Pinned bool
	// AgentType is the subagent type the harness reports (for example a Claude
	// Code subagent_type); empty when absent. Profiles may match on it.
	AgentType string
	// Profile is the profile the caller named, if any. A profile is the only
	// source of what it sets (REA section 2): a request that names a profile
	// and also pins a model (Pinned) is refused as a config error (ReasonProfileOverride).
	Profile ProfileName
	Hint    Hint
}

func member[T comparable](v T, set []T) bool {
	for _, s := range set {
		if v == s {
			return true
		}
	}
	return false
}
