// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package classify

import (
	"os"
	"regexp"
)

// DomainSignalDef identifies a task-specific lexical signal for review,
// debugging and future supervised weight calibration. Domain is metadata;
// a domain name by itself never determines task complexity or model choice.
type DomainSignalDef struct {
	Domain string
	ID     string
	SignalDef
}

// RawDomainSignals is the growing Go-native domain vocabulary. Every signal
// pairs an action with a technical context instead of promoting a task just
// because a framework or cloud provider is mentioned.
//
// Evolve via reviewed misroute examples, regression cases and holdout evals.
// Keep the production baseline in signals.go unchanged while validating
// this opt-in layer (DOWNSHIFT_DOMAIN_SIGNALS=1).
var RawDomainSignals = []DomainSignalDef{
	{Domain: "frontend", ID: "fe-hydration-debug", SignalDef: SignalDef{
		Pattern: `\b(?:debug|investigat\w*|corrig\w*|depur\w*|fix)\b.{0,80}\b(?:hydration mismatch|hidrata[cç][aã]o|lcp|cls|inp|web vitals)\b`,
		Weight: 2, Class: Medium,
	}},
	{Domain: "frontend", ID: "fe-component-integration", SignalDef: SignalDef{
		Pattern: `\b(?:integrat\w*|integrar|integre|connect|conectar|sincroniz\w*)\b.{0,80}\b(?:react|vue|angular|front.?end|ui|component|componente)\b.{0,60}\b(?:api|state|estado|backend|back.?end)\b`,
		Weight: 2, Class: Medium,
	}},
	{Domain: "frontend", ID: "fe-design-system-migration", SignalDef: SignalDef{
		Pattern: `\b(?:migrat\w*|migrar|redesign|redesenh\w*|rebuild|reconstru\w*)\b.{0,100}\b(?:design system|micro.frontends?|server components?|rendering architecture|arquitetura de renderiza[cç][aã]o)\b`,
		Weight: 3, Class: Complex,
	}},
	{Domain: "backend", ID: "be-pagination-endpoint", SignalDef: SignalDef{
		Pattern: `\b(?:implement\w*|implementar|implemente|add|adicion\w*|criar|crie)\b.{0,80}\b(?:cursor pagination|pagina[cç][aã]o|rate limiting|endpoint pagination|paginat\w*)\b`,
		Weight: 2, Class: Medium,
	}},
	{Domain: "backend", ID: "be-query-profiling", SignalDef: SignalDef{
		Pattern: `\b(?:debug|investigat\w*|diagnos\w*|corrig\w*|fix|optimis\w*|optimiz\w*|otimiz\w*)\b.{0,80}\b(?:n\+1|slow quer\w*|consulta lenta|query plan|execution plan|plano de execu[cç][aã]o)\b`,
		Weight: 2, Class: Medium,
	}},
	{Domain: "backend", ID: "be-consistency-design", SignalDef: SignalDef{
		Pattern: `\b(?:design|projet\w*|desenh\w*|implement\w*|implementar)\b.{0,100}\b(?:transactional outbox|saga pattern|exactly.once|eventual consistency|consist[eê]ncia eventual|distributed transaction|transa[cç][aã]o distribu)\w*`,
		Weight: 3, Class: Complex,
	}},
	{Domain: "fullstack", ID: "fs-api-ui-feature", SignalDef: SignalDef{
		Pattern: `\b(?:implement\w*|implementar|constru\w*|build|integrat\w*|integrar)\b.{0,100}\b(?:ui|frontend|front.?end|interface|tela)\b.{0,100}\b(?:backend|back.?end|api|database|banco de dados)\b`,
		Weight: 2, Class: Medium,
	}},
	{Domain: "fullstack", ID: "fs-end-to-end-feature", SignalDef: SignalDef{
		Pattern: `\b(?:feature|funcionalidade)\b.{0,70}\b(?:end.to.end|ponta a ponta|front.?end.{0,30}back.?end|api.{0,30}ui)\b`,
		Weight: 2, Class: Medium,
	}},
	{Domain: "fullstack", ID: "fs-coordinated-rollout", SignalDef: SignalDef{
		Pattern: `\b(?:coordinate|coorden\w*|orchestr\w*|orquestr\w*|plan|planej\w*)\b.{0,80}\b(?:rollout|deploy|implant\w*|lan[cç]amento)\b.{0,100}\b(?:schema|database|banco de dados)\b.{0,100}\b(?:frontend|front.?end|client|clientes)\b`,
		Weight: 3, Class: Complex,
	}},
	{Domain: "devops", ID: "ops-crashloop-debug", SignalDef: SignalDef{
		Pattern: `\b(?:debug|investigat\w*|troubleshoot|diagnos\w*|corrig\w*|fix)\b.{0,80}\b(?:crashloopbackoff|oomkilled|pod eviction|evic[cç][aã]o de pods|kubernetes scheduling)\b`,
		Weight: 2, Class: Medium,
	}},
	{Domain: "devops", ID: "ops-helm-automation", SignalDef: SignalDef{
		Pattern: `\b(?:automate|automatiz\w*|implement\w*|implementar)\b.{0,80}\b(?:helm chart|terraform module|m[oó]dulo terraform|kubernetes operator)\b`,
		Weight: 2, Class: Medium,
	}},
	{Domain: "devops", ID: "ops-multi-region-recovery", SignalDef: SignalDef{
		Pattern: `\b(?:design|projet\w*|implement\w*|implementar|rearchitect|re.?arquitet\w*)\b.{0,100}\b(?:disaster recovery|recupera[cç][aã]o de desastres|multi.region failover|failover multirregional|active.active)\b`,
		Weight: 3, Class: Complex,
	}},
	{Domain: "finops", ID: "finops-anomaly-investigation", SignalDef: SignalDef{
		Pattern: `\b(?:investigat\w*|analis\w*|analy[sz]\w*|diagnos\w*)\b.{0,90}\b(?:cloud cost anomal\w*|anomalia de custos?|unexpected spend|gasto inesperado|egress cost|custo de egress)\b`,
		Weight: 2, Class: Medium,
	}},
	{Domain: "finops", ID: "finops-rightsizing-plan", SignalDef: SignalDef{
		Pattern: `\b(?:optimiz\w*|optimis\w*|otimiz\w*|rightsiz\w*|dimension\w*)\b.{0,90}\b(?:cloud cost|custos? de nuvem|compute spend|reserved instances|inst[aâ]ncias reservadas|savings plans?)\b`,
		Weight: 2, Class: Medium,
	}},
	{Domain: "finops", ID: "finops-chargeback-design", SignalDef: SignalDef{
		Pattern: `\b(?:design|projet\w*|implement\w*|implementar|architect\w*|arquitet\w*)\b.{0,100}\b(?:multi.account chargeback|rateio de custos entre contas|showback model|cost allocation across accounts|aloca[cç][aã]o de custos entre contas)\b`,
		Weight: 3, Class: Complex,
	}},
	{Domain: "devsecops", ID: "sec-sast-triage", SignalDef: SignalDef{
		Pattern: `\b(?:triage|investigat\w*|corrig\w*|remediat\w*|remedi\w*|fix)\b.{0,100}\b(?:sast|dast|dependency scanning|varredura de depend[eê]ncias|secret scanning|scan de segredos)\b`,
		Weight: 2, Class: Medium,
	}},
	{Domain: "devsecops", ID: "sec-supply-chain-hardening", SignalDef: SignalDef{
		Pattern: `\b(?:implement\w*|implementar|harden|hardening|endurec\w*|proteger)\b.{0,100}\b(?:slsa|sbom|provenance attestations?|atestados? de proveni[eê]ncia|sigstore|cosign)\b`,
		Weight: 2, Class: Medium,
	}},
	{Domain: "devsecops", ID: "sec-threat-model-design", SignalDef: SignalDef{
		Pattern: `\b(?:design|projet\w*|create|criar|implementar|implement\w*)\b.{0,100}\b(?:threat model|modelo de amea[cç]as|zero trust architecture|arquitetura zero trust|trust boundar\w*|fronteiras? de confian[cç]a)\b`,
		Weight: 3, Class: Complex,
	}},
	{Domain: "mobile", ID: "mob-deep-link-integration", SignalDef: SignalDef{
		Pattern: `\b(?:implement\w*|implementar|integrat\w*|integrar|configur\w*)\b.{0,90}\b(?:deep links?|universal links?|app links?|navega[cç][aã]o profunda)\b`,
		Weight: 2, Class: Medium,
	}},
	{Domain: "mobile", ID: "mob-anr-crash-debug", SignalDef: SignalDef{
		Pattern: `\b(?:debug|investigat\w*|diagnos\w*|corrig\w*|fix)\b.{0,90}\b(?:android anr|anr|ios crash|app crash|crash no aplicativo|memory pressure)\b`,
		Weight: 2, Class: Medium,
	}},
	{Domain: "mobile", ID: "mob-offline-conflict-design", SignalDef: SignalDef{
		Pattern: `\b(?:design|projet\w*|implement\w*|implementar|redesign|redesenh\w*)\b.{0,100}\b(?:offline.first sync|offline sync conflict|conflict resolution offline|resolu[cç][aã]o de conflitos offline|cross.device state replication)\b`,
		Weight: 3, Class: Complex,
	}},
	{Domain: "cicd", ID: "ci-flaky-test-repair", SignalDef: SignalDef{
		Pattern: `\b(?:fix|corrig\w*|investigat\w*|debug|diagnos\w*)\b.{0,80}\b(?:flaky tests?|testes? intermitentes?|pipeline flak\w*|instabilidade do pipeline)\b`,
		Weight: 2, Class: Medium,
	}},
	{Domain: "cicd", ID: "ci-build-matrix", SignalDef: SignalDef{
		Pattern: `\b(?:implement\w*|implementar|build|criar|crie|configur\w*)\b.{0,80}\b(?:build matrix|matriz de builds?|test matrix|matriz de testes?|parallel jobs|jobs paralelos)\b`,
		Weight: 2, Class: Medium,
	}},
	{Domain: "cicd", ID: "ci-progressive-delivery", SignalDef: SignalDef{
		Pattern: `\b(?:design|projet\w*|implement\w*|implementar|orchestrat\w*|orquestr\w*)\b.{0,100}\b(?:progressive delivery|canary rollout|blue.green deployment|automated rollback|rollback autom[aá]tico|deploy progressivo)\b`,
		Weight: 3, Class: Complex,
	}},
}

// Compiled once, reused on all routing calls. No parsing, network or LLM call
// runs inside the hook, and the domain vocabulary has no external runtime files.
var enrichedSignals = compileEnrichedSignals()

func compileEnrichedSignals() []signal {
	out := make([]signal, 0, len(signals)+len(RawDomainSignals))
	out = append(out, signals...)
	for _, s := range RawDomainSignals {
		out = append(out, signal{
			re: regexp.MustCompile(`(?i)` + s.Pattern),
			weight: s.Weight,
			class: s.Class,
		})
	}
	return out
}

// activeSignals integrates domain vocabulary with the existing weighted-vote,
// long-brief, task-focus, negation and risk-floor pipeline. The new layer is
// opt-in until it passes existing regressions and reviewed holdout evaluations.
func activeSignals() []signal {
	if os.Getenv("DOWNSHIFT_DOMAIN_SIGNALS") == "1" {
		return enrichedSignals
	}
	return signals
}
