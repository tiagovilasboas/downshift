// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package contract

// Complexity is the serialized classifier class a routing rule may admit. The
// strings match core.Classification's Complexity.String ("TRIVIAL", "SIMPLE",
// "MEDIUM", "COMPLEX"), so a policy file and telemetry keep one spelling. The
// classifier is telemetry and the key for user-declared rules, never a decider
// on its own (owner decision O8): a rule admits a class only when the user
// lists it, the classification is confident and the task has no risk.
//
// The contract does not reuse core.Complexity here so routing/policy keeps its
// one import of core; the value is converted from the class string the caller
// already holds (decide.Input.Class).
type Complexity string

const (
	ComplexityUnknown Complexity = ""
	ComplexityTrivial Complexity = "TRIVIAL"
	ComplexitySimple  Complexity = "SIMPLE"
	ComplexityMedium  Complexity = "MEDIUM"
	ComplexityComplex Complexity = "COMPLEX"
)

// Complexities lists every known Complexity from cheapest to hardest.
func Complexities() []Complexity {
	return []Complexity{ComplexityTrivial, ComplexitySimple, ComplexityMedium, ComplexityComplex}
}

// Valid reports whether c is a known class. The empty value is not: an unknown
// class never admits a rule.
func (c Complexity) Valid() bool { return member(c, Complexities()) }
