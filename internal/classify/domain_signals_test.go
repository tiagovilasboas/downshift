// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package classify

import (
	"regexp"
	"testing"
)

func TestDomainSignalsIntegrity(t *testing.T) {
	seen := make(map[string]bool)
	domains := make(map[string]bool)
	for _, row := range RawDomainSignals {
		if row.ID == "" || row.Domain == "" || seen[row.ID] {
			t.Fatalf("invalid or duplicate signal: %+v", row)
		}
		seen[row.ID] = true
		domains[row.Domain] = true
		if row.Weight < 1 || row.Weight > 3 {
			t.Fatalf("%s invalid weight: %d", row.ID, row.Weight)
		}
		if row.Class != Medium && row.Class != Complex {
			t.Fatalf("%s unexpected class: %s", row.ID, row.Class)
		}
		if _, err := regexp.Compile("(?i)" + row.Pattern); err != nil {
			t.Fatalf("%s invalid regex: %v", row.ID, err)
		}
	}
	if len(domains) != 8 || len(RawDomainSignals) != 32 {
		t.Fatalf("expected 8 domains / 32 rules, got %d domains / %d rules", len(domains), len(RawDomainSignals))
	}
}

// Bilingual parity is mandatory for every domain rule: EN, PT-BR and mixed
// examples must all trigger the SAME rule, complexity and weight. This guards
// against silently dropping Portuguese verb forms during lexical tuning.
func TestDomainSignalsBilingualParity(t *testing.T) {
	examples := []struct{ id, en, pt, mixed string }{
		{"fe-hydration-debug", "Debug hydration mismatch in React SSR", "Corrija a hidratação na aplicação React", "Investigue hydration mismatch no React"},
		{"fe-component-integration", "Integrate React component with backend API", "Integre componente React com API do backend", "Conecte UI ao backend via API"},
		{"fe-design-system-migration", "Migrate the design system", "Migre o sistema de design", "Redesenhe arquitetura de renderização"},
		{"fe-typescript-typecheck", "Fix TypeScript typecheck failures in TSX", "Corrija erros de tipagem no TypeScript", "Investigue typecheck no TSX"},
		{"be-pagination-endpoint", "Implement cursor pagination", "Implemente paginação por cursor", "Crie cursor pagination na API"},
		{"be-query-profiling", "Diagnose slow query", "Diagnostique consulta lenta", "Corrija N+1 no banco"},
		{"be-consistency-design", "Design transactional outbox", "Projete consistência eventual com transactional outbox", "Desenhe transação distribuída"},
		{"be-api-contract-versioning", "Plan API contract versioning", "Planeje versionamento de contrato da API", "Projete backward compatibility no OpenAPI"},
		{"fs-api-ui-feature", "Implement UI and API", "Implemente tela e API", "Integre front-end e backend"},
		{"fs-end-to-end-feature", "Build feature end-to-end", "Crie funcionalidade de ponta a ponta", "Implemente feature ponta a ponta"},
		{"fs-coordinated-rollout", "Plan rollout schema and frontend", "Planeje implantação do schema e frontend", "Coordene deploy database e client"},
		{"fs-contract-compatibility", "Implement contract compatibility between client and server", "Implemente compatibilidade de contrato entre cliente e servidor", "Crie backward compatible API frontend"},
		{"ops-crashloop-debug", "Investigate CrashLoopBackOff", "Investigue pods reiniciando por OOMKilled", "Corrija crashloopbackoff no cluster"},
		{"ops-helm-automation", "Automate Helm chart provisioning", "Automatize módulo Terraform", "Configure kubernetes operator"},
		{"ops-multi-region-recovery", "Design disaster recovery", "Projete recuperação de desastres", "Implemente multi-region failover"},
		{"ops-terraform-drift", "Investigate Terraform drift", "Investigue drift de infraestrutura", "Diagnostique configuration drift"},
		{"finops-anomaly-investigation", "Investigate cloud cost anomaly", "Investigue anomalia de custos", "Analise gasto inesperado de egress"},
		{"finops-rightsizing-plan", "Optimize reserved instances", "Otimize custos de nuvem com instâncias reservadas", "Dimensione savings plans"},
		{"finops-chargeback-design", "Design multi-account chargeback", "Projete rateio de custos entre contas", "Implemente cost allocation across accounts"},
		{"finops-amortized-cost", "Analyze amortized cost", "Analise custo amortizado", "Investigue effective cost na fatura"},
		{"sec-sast-triage", "Investigate SAST findings", "Investigue análise estática de segurança", "Corrija falhas no DAST"},
		{"sec-supply-chain-hardening", "Implement SBOM provenance", "Implemente atestados de proveniência", "Configure sigstore cosign"},
		{"sec-threat-model-design", "Design threat model", "Projete modelo de ameaças", "Implemente arquitetura zero trust"},
		{"sec-asvs-access-control", "Investigate broken access control", "Investigue falha de controle de acesso", "Corrija IDOR"},
		{"mob-deep-link-integration", "Implement deep link navigation", "Implemente links universais", "Configure deep links"},
		{"mob-anr-crash-debug", "Investigate iOS crash", "Investigue travamento no aplicativo Android", "Corrija ANR"},
		{"mob-offline-conflict-design", "Design offline-first sync", "Projete sincronização offline com resolução de conflitos", "Implemente offline sync conflict resolution"},
		{"mob-ui-lifecycle-debug", "Debug lifecycle issue in SwiftUI", "Corrija problema de ciclo de vida no Jetpack Compose", "Investigue lifecycle no SwiftUI"},
		{"ci-flaky-test-repair", "Fix flaky tests", "Corrija testes intermitentes", "Investigue pipeline flaky"},
		{"ci-build-matrix", "Configure build matrix", "Configure matriz de builds", "Crie parallel jobs"},
		{"ci-progressive-delivery", "Design canary rollout", "Projete deploy progressivo", "Implemente automated rollback"},
		{"ci-cache-invalidation", "Diagnose pipeline cache invalidation", "Investigue invalidação de cache da pipeline", "Corrija cache misses no CI"},
	}
	if len(examples) != len(RawDomainSignals) {
		t.Fatalf("coverage incomplete: %d bilingual groups / %d rules", len(examples), len(RawDomainSignals))
	}
	byID := make(map[string]DomainSignalDef)
	for _, s := range RawDomainSignals {
		byID[s.ID] = s
	}
	for _, pair := range examples {
		t.Run(pair.id, func(t *testing.T) {
			signal, ok := byID[pair.id]
			if !ok {
				t.Fatalf("missing rule for bilingual pair %s", pair.id)
			}
			re := regexp.MustCompile("(?i)" + signal.Pattern)
			for _, prompt := range []struct{ lang, text string }{
				{"en", pair.en}, {"pt-BR", pair.pt}, {"mixed", pair.mixed},
			} {
				if !re.MatchString(prompt.text) {
					t.Errorf("no match for %s: %q", prompt.lang, prompt.text)
				}
			}
		})
	}
}

// Technical nouns alone must not trigger an enriched domain signal. Complexity
// continues to depend on the requested action plus bounded technical context.
func TestDomainSignalsNoBareTechnologyPromotions(t *testing.T) {
	for _, prompt := range []string{
		"React", "Terraform", "Kubernetes", "SwiftUI", "TypeScript",
		"SAST", "OWASP", "FOCUS", "Helm", "Postgres", "CI/CD",
		"Explique o que é React.", "Summarize Kubernetes docs.",
	} {
		for _, rule := range RawDomainSignals {
			if regexp.MustCompile("(?i)" + rule.Pattern).MatchString(prompt) {
				t.Errorf("technical mention %q unexpectedly triggered %s", prompt, rule.ID)
			}
		}
	}
}

func TestDomainSignalsOptIn(t *testing.T) {
	t.Setenv("DOWNSHIFT_DOMAIN_SIGNALS", "0")
	if len(activeSignals()) != len(signals) {
		t.Fatal("default must use baseline")
	}
	t.Setenv("DOWNSHIFT_DOMAIN_SIGNALS", "1")
	if len(activeSignals()) != len(signals)+len(RawDomainSignals) {
		t.Fatal("opt-in must append domain rules")
	}
	t.Setenv("DOWNSHIFT_DOMAIN_SIGNALS", "0")
	if len(activeSignals()) != len(signals) {
		t.Fatal("opt-out must restore baseline")
	}
}
