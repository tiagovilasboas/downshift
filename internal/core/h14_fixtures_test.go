// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package core_test

// Downshift by Tiago Vilas Boas
// https://github.com/tiagovilasboas/downshift

// Canonical M3 / H14 evaluation prompts (downshift-labs multilang m3-v1).
// Single source for acceptance and unit tests in this package.

const h14DelegationBriefPrompt = "You own the bounded authorization boundary workstream. Read src/handler.go, src/service.go, src/repository.go and their adjacent tests before editing. Implement the requested behavior using existing interfaces. Other engineers are working in the repository; preserve their edits. Stay within these three modules, explain any compatibility change, and run the relevant tests. Return the changed files, evidence of the behavior, and unresolved concerns. Do not publish, deploy, or modify unrelated subsystems. Acceptance case delegation-16: return only the bounded module change and a focused regression."

func h14MixedRiskPrompts() []string {
	return []string{
		"rename this helper; Revisa los permisos de acceso a los datos de los clientes.",
		"rename this helper; Lösche personenbezogene Kundendaten gemäß Aufbewahrungsrichtlinie.",
		"format this helper; Behebe SQL-Injektion in der Kundensuche.",
	}
}

func h14ParallelHighStakesPrompts() []string {
	return []string{
		"Implement idempotent payment refunds with audit evidence.",
		"Revise as permissões de acesso aos dados dos clientes.",
	}
}

func h14PureEnglishTrivialPrompts() []string {
	return []string{
		"rename the variable userId to userIdentifier",
		"rename foo to bar",
		"format main.go",
	}
}

func h14UnsafeBucketPrompts() []string {
	out := append([]string{}, h14MixedRiskPrompts()...)
	out = append(out, h14DelegationBriefPrompt)
	out = append(out, h14ParallelHighStakesPrompts()...)
	return out
}
