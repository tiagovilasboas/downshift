// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package classify

import (
	"os"
	"regexp"
)

// DomainSignalDef identifies a task-specific signal for audit and ongoing
// calibration. Domain never determines complexity or model selection by itself.
type DomainSignalDef struct {
	Domain string
	ID     string
	SignalDef
}

// RawDomainSignals is independent of the stable production signal table.
// Synonyms are curated from terminology documented by Linguist, OWASP ASVS,
// Stack Overflow, and FinOps FOCUS; this is NOT a copy of external rule sets.
// Every rule references reviewed English and PT-BR terms embedded as JSON.\n// Every row needs bilingual regression coverage.
//
// Source references:
// https://github.com/github-linguist/linguist
// https://github.com/OWASP/ASVS
// https://focus.finops.org/docs/specification/v1-2/glossary/
// https://api.stackexchange.com/docs/synonyms-by-tags
//
// No model ids or model scores belong in this table. Evals may propose
// weight changes; promotion requires independent reviewed holdout evidence.
var RawDomainSignals = []DomainSignalDef{
	{Domain: "frontend", ID: "fe-hydration-debug", SignalDef: SignalDef{
		Pattern: domainPatternFromLexicon("fe-hydration-debug", "inspect"),
		Weight:  2, Class: Medium,
	}},
	{Domain: "frontend", ID: "fe-component-integration", SignalDef: SignalDef{
		Pattern: domainPatternFromLexicon("fe-component-integration", "integrate"),
		Weight:  2, Class: Medium,
	}},
	{Domain: "frontend", ID: "fe-design-system-migration", SignalDef: SignalDef{
		Pattern: domainPatternFromLexicon("fe-design-system-migration", "plan"),
		Weight:  3, Class: Complex,
	}},
	{Domain: "frontend", ID: "fe-typescript-typecheck", SignalDef: SignalDef{
		Pattern: domainPatternFromLexicon("fe-typescript-typecheck", "fix"),
		Weight:  2, Class: Medium,
	}},
	{Domain: "backend", ID: "be-pagination-endpoint", SignalDef: SignalDef{
		Pattern: domainPatternFromLexicon("be-pagination-endpoint", "build"),
		Weight:  2, Class: Medium,
	}},
	{Domain: "backend", ID: "be-query-profiling", SignalDef: SignalDef{
		Pattern: domainPatternFromLexicon("be-query-profiling", "inspect"),
		Weight:  2, Class: Medium,
	}},
	{Domain: "backend", ID: "be-consistency-design", SignalDef: SignalDef{
		Pattern: domainPatternFromLexicon("be-consistency-design", "plan"),
		Weight:  3, Class: Complex,
	}},
	{Domain: "backend", ID: "be-api-contract-versioning", SignalDef: SignalDef{
		Pattern: domainPatternFromLexicon("be-api-contract-versioning", "plan"),
		Weight:  2, Class: Medium,
	}},
	{Domain: "fullstack", ID: "fs-api-ui-feature", SignalDef: SignalDef{
		Pattern: domainPatternFromLexicon("fs-api-ui-feature", "build"),
		Weight:  2, Class: Medium,
	}},
	{Domain: "fullstack", ID: "fs-end-to-end-feature", SignalDef: SignalDef{
		Pattern: domainPatternFromLexicon("fs-end-to-end-feature", "build"),
		Weight:  2, Class: Medium,
	}},
	{Domain: "fullstack", ID: "fs-coordinated-rollout", SignalDef: SignalDef{
		Pattern: domainPatternFromLexicon("fs-coordinated-rollout", "plan"),
		Weight:  3, Class: Complex,
	}},
	{Domain: "fullstack", ID: "fs-contract-compatibility", SignalDef: SignalDef{
		Pattern: domainPatternFromLexicon("fs-contract-compatibility", "build"),
		Weight:  2, Class: Medium,
	}},
	{Domain: "devops", ID: "ops-crashloop-debug", SignalDef: SignalDef{
		Pattern: domainPatternFromLexicon("ops-crashloop-debug", "inspect"),
		Weight:  2, Class: Medium,
	}},
	{Domain: "devops", ID: "ops-helm-automation", SignalDef: SignalDef{
		Pattern: domainPatternFromLexicon("ops-helm-automation", "automate"),
		Weight:  2, Class: Medium,
	}},
	{Domain: "devops", ID: "ops-multi-region-recovery", SignalDef: SignalDef{
		Pattern: domainPatternFromLexicon("ops-multi-region-recovery", "plan"),
		Weight:  3, Class: Complex,
	}},
	{Domain: "devops", ID: "ops-terraform-drift", SignalDef: SignalDef{
		Pattern: domainPatternFromLexicon("ops-terraform-drift", "inspect"),
		Weight:  2, Class: Medium,
	}},
	{Domain: "finops", ID: "finops-anomaly-investigation", SignalDef: SignalDef{
		Pattern: domainPatternFromLexicon("finops-anomaly-investigation", "inspect"),
		Weight:  2, Class: Medium,
	}},
	{Domain: "finops", ID: "finops-rightsizing-plan", SignalDef: SignalDef{
		Pattern: domainPatternFromLexicon("finops-rightsizing-plan", "optimize"),
		Weight:  2, Class: Medium,
	}},
	{Domain: "finops", ID: "finops-chargeback-design", SignalDef: SignalDef{
		Pattern: domainPatternFromLexicon("finops-chargeback-design", "plan"),
		Weight:  3, Class: Complex,
	}},
	{Domain: "finops", ID: "finops-amortized-cost", SignalDef: SignalDef{
		Pattern: domainPatternFromLexicon("finops-amortized-cost", "inspect"),
		Weight:  2, Class: Medium,
	}},
	{Domain: "devsecops", ID: "sec-sast-triage", SignalDef: SignalDef{
		Pattern: domainPatternFromLexicon("sec-sast-triage", "inspect"),
		Weight:  2, Class: Medium,
	}},
	{Domain: "devsecops", ID: "sec-supply-chain-hardening", SignalDef: SignalDef{
		Pattern: domainPatternFromLexicon("sec-supply-chain-hardening", "protect"),
		Weight:  2, Class: Medium,
	}},
	{Domain: "devsecops", ID: "sec-threat-model-design", SignalDef: SignalDef{
		Pattern: domainPatternFromLexicon("sec-threat-model-design", "plan"),
		Weight:  3, Class: Complex,
	}},
	{Domain: "devsecops", ID: "sec-asvs-access-control", SignalDef: SignalDef{
		Pattern: domainPatternFromLexicon("sec-asvs-access-control", "inspect"),
		Weight:  2, Class: Medium,
	}},
	{Domain: "mobile", ID: "mob-deep-link-integration", SignalDef: SignalDef{
		Pattern: domainPatternFromLexicon("mob-deep-link-integration", "build"),
		Weight:  2, Class: Medium,
	}},
	{Domain: "mobile", ID: "mob-anr-crash-debug", SignalDef: SignalDef{
		Pattern: domainPatternFromLexicon("mob-anr-crash-debug", "inspect"),
		Weight:  2, Class: Medium,
	}},
	{Domain: "mobile", ID: "mob-offline-conflict-design", SignalDef: SignalDef{
		Pattern: domainPatternFromLexicon("mob-offline-conflict-design", "plan"),
		Weight:  3, Class: Complex,
	}},
	{Domain: "mobile", ID: "mob-ui-lifecycle-debug", SignalDef: SignalDef{
		Pattern: domainPatternFromLexicon("mob-ui-lifecycle-debug", "inspect"),
		Weight:  2, Class: Medium,
	}},
	{Domain: "cicd", ID: "ci-flaky-test-repair", SignalDef: SignalDef{
		Pattern: domainPatternFromLexicon("ci-flaky-test-repair", "fix"),
		Weight:  2, Class: Medium,
	}},
	{Domain: "cicd", ID: "ci-build-matrix", SignalDef: SignalDef{
		Pattern: domainPatternFromLexicon("ci-build-matrix", "automate"),
		Weight:  2, Class: Medium,
	}},
	{Domain: "cicd", ID: "ci-progressive-delivery", SignalDef: SignalDef{
		Pattern: domainPatternFromLexicon("ci-progressive-delivery", "plan"),
		Weight:  3, Class: Complex,
	}},
	{Domain: "cicd", ID: "ci-cache-invalidation", SignalDef: SignalDef{
		Pattern: domainPatternFromLexicon("ci-cache-invalidation", "fix"),
		Weight:  2, Class: Medium,
	}},
}

// Compile once, with no runtime file parsing, network access or LLM call.
var enrichedSignals []signal

func init() {
	enrichedSignals = compileEnrichedSignals()
}

func compileEnrichedSignals() []signal {
	out := make([]signal, 0, len(signals)+len(RawDomainSignals)+len(RawExtendedDomainSignals))
	out = append(out, signals...)
	for _, table := range [][]DomainSignalDef{RawDomainSignals, RawExtendedDomainSignals} {
		for _, s := range table {
			out = append(out, signal{
				re:     regexp.MustCompile(`(?i)` + s.Pattern),
				weight: s.Weight,
				class:  s.Class,
				domain: s.Domain,
			})
		}
	}
	return out
}

// Enabled by default. Set DOWNSHIFT_DOMAIN_SIGNALS=0 to restore the
// original production rule table without this domain enrichment.
func activeSignals() []signal {
	if os.Getenv("DOWNSHIFT_DOMAIN_SIGNALS") == "0" {
		return signals
	}
	return enrichedSignals
}
