// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package classify

import "testing"

func TestDomainEvidenceSaturation(t *testing.T) {
	t.Setenv("DOWNSHIFT_DOMAIN_INTERACTIONS", "0")
	t.Setenv("DOWNSHIFT_DOMAIN_DEDUP", "1")
	var e domainEvidence
	e.add("backend", Medium, 2)
	e.add("backend", Medium, 2)
	e.add("backend", Medium, 2)
	e.add("frontend", Medium, 3)
	e.add("backend", Complex, 3)
	scores := map[Complexity]int{Medium: 5}
	e.apply(scores)
	// backend mid: 2+1 = 3, frontend mid: 3, generic mid: 5.
	if scores[Medium] != 11 || scores[Complex] != 3 {
		t.Fatalf("unexpected saturated scores: %+v", scores)
	}
}

func TestDomainEvidenceWithoutDuplicates(t *testing.T) {
	t.Setenv("DOWNSHIFT_DOMAIN_INTERACTIONS", "0")
	t.Setenv("DOWNSHIFT_DOMAIN_DEDUP", "1")
	var e domainEvidence
	e.add("mobile", Complex, 3)
	scores := map[Complexity]int{}
	e.apply(scores)
	if scores[Complex] != 3 {
		t.Fatalf("single domain evidence must remain unchanged: %+v", scores)
	}
}

func TestDomainEvidenceOptOutRestoresRawWeights(t *testing.T) {
	t.Setenv("DOWNSHIFT_DOMAIN_INTERACTIONS", "0")
	t.Setenv("DOWNSHIFT_DOMAIN_DEDUP", "0")
	var e domainEvidence
	e.add("backend", Medium, 2)
	e.add("backend", Medium, 2)
	e.add("backend", Medium, 2)
	scores := map[Complexity]int{}
	e.apply(scores)
	if scores[Medium] != 6 {
		t.Fatalf("opt-out must preserve additive weights: %+v", scores)
	}
}
