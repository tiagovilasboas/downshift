// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package classify

import "os"

func osDomainDedupEnabled() bool {
	return os.Getenv("DOWNSHIFT_DOMAIN_DEDUP") != "0"
}

// domainEvidenceKey groups overlapping signals from one specialty and class.
// General signals and risk floors are deliberately not aggregated.
type domainEvidenceKey struct {
	domain string
	class  Complexity
}

type domainEvidenceVote struct {
	total int
	peak  int
}

// domainEvidence applies a bounded corroboration bonus within a domain.
// Five closely related regex hits are not five independent observations:
//     contribution = max(weight) + min(1, sum(weight)-max(weight))
// One hit is unchanged; two or more contribute at most one extra point.
// This is a conservative saturation heuristic, not a calibrated probability.
type domainEvidence struct {
	groups map[domainEvidenceKey]domainEvidenceVote
}

func (d *domainEvidence) add(domain string, class Complexity, weight int) {
	if d.groups == nil {
		d.groups = make(map[domainEvidenceKey]domainEvidenceVote)
	}
	key := domainEvidenceKey{domain: domain, class: class}
	vote := d.groups[key]
	vote.total += weight
	if weight > vote.peak {
		vote.peak = weight
	}
	d.groups[key] = vote
}

func (d *domainEvidence) apply(scores map[Complexity]int) {
	// Explicit opt-out lets benchmark comparisons isolate this mathematical
	// change without disabling the bilingual dictionary or risk policies.
	raw := !osDomainDedupEnabled()
	for key, vote := range d.groups {
		value := vote.total
		if !raw {
			value = vote.peak
			if vote.total > vote.peak {
				value++
			}
		}
		scores[key.class] += value
	}
}
