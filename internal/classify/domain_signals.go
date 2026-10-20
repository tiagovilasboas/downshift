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

// Shared verbs mirror English and Brazilian Portuguese phrasing. They are
// action words, not framework names. All expressions use Go's RE2 syntax.
const (
	domainActInspect   = `(?:debug\w*|investig\w*|troubleshoot|diagnos\w*|diagn[oó]stic\w*|corrig\w*|corrij\w*|fix|depur\w*|analy[sz]\w*|analis\w*|an[aá]lis\w*)`
	domainActIntegrate = `(?:integrat\w*|integr\w*|connect\w*|conect\w*|sincroniz\w*|implement\w*|implementar|implemente)`
	domainActBuild     = `(?:implement\w*|implementa[cç][aã]o|criar|crie|cria|create|build|constru\w*|configur\w*|configure|integr\w*|conect\w*|add|adicion\w*)`
	domainActPlan      = `(?:design\w*|desenh\w*|redesenh\w*|arquitet\w*|rearquitet\w*|architect\w*|projet\w*|plan\w*|planej\w*|implement\w*|implementar|implemente|rebuild|reconstru\w*|migrat\w*|migrar|migre|migra\w*|orchestrat\w*|orquestr\w*|coorden\w*|coordinat\w*)`
	domainActAutomate  = `(?:automat\w*|automate|automatiz\w*|configure|configur\w*|provision\w*|implement\w*|implementar|implemente|create|criar|crie|build)`
	domainActOptimize  = `(?:optimi[sz]\w*|otimiz\w*|dimension\w*|rightsiz\w*|ajust\w*|reduz\w*|reduce)`
	domainActProtect   = `(?:implement\w*|implementar|implemente|harden\w*|endurec\w*|proteg\w*|protect\w*|configur\w*|configure|assinar|sign)`
	domainActFix       = `(?:fix|corrig\w*|corrij\w*|investig\w*|debug\w*|diagnos\w*|diagn[oó]stic\w*|repair|repar\w*)`
)

// An action and a technical target must be in the same short clause, not
// separated by a paragraph of background. The action comes first.
// Use \b only around words whose first/last characters are ASCII word chars;
// technical expressions may have accents internally, e.g. "sincronização".
func domainPattern(action, target string) string {
	return `\b` + action + `\b[^;\n]{0,110}\b(?:` + target + `)\b`
}

// RawDomainSignals is independent of the stable production signal table.
// Synonyms are curated from terminology documented by Linguist, OWASP ASVS,
// Stack Overflow, and FinOps FOCUS; this is NOT a copy of external rule sets.
// Every row is EN/PT-BR and must have a reviewed bilingual regression pair.
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
		Pattern: domainPattern(domainActInspect, `hydration mismatch|hidrata[cç][aã]o|lcp|cls|inp|web vitals`),
		Weight:  2, Class: Medium,
	}},
	{Domain: "frontend", ID: "fe-component-integration", SignalDef: SignalDef{
		Pattern: domainPattern(domainActIntegrate, `(?:react|vue|angular|front[ -]?end|ui|component\w*|componentes?|interface|tela).{0,70}(?:api|state|estado|back[ -]?end|servidor)`),
		Weight:  2, Class: Medium,
	}},
	{Domain: "frontend", ID: "fe-design-system-migration", SignalDef: SignalDef{
		Pattern: domainPattern(domainActPlan, `design system|sistema de design|micro[ -]?frontends?|server components?|componentes? de servidor|rendering architecture|arquitetura de renderiza[cç][aã]o`),
		Weight:  3, Class: Complex,
	}},
	{Domain: "frontend", ID: "fe-typescript-typecheck", SignalDef: SignalDef{
		Pattern: domainPattern(domainActFix, `typescript type errors?|typecheck|tsx|erros? de tipagem|falhas? de tipagem|checagem de tipos`),
		Weight:  2, Class: Medium,
	}},
	{Domain: "backend", ID: "be-pagination-endpoint", SignalDef: SignalDef{
		Pattern: domainPattern(domainActBuild, `cursor pagination|pagina[cç][aã]o por cursor|pagina[cç][aã]o|rate limit(?:ing)?|limite de requisi[cç][oõ]es`),
		Weight:  2, Class: Medium,
	}},
	{Domain: "backend", ID: "be-query-profiling", SignalDef: SignalDef{
		Pattern: domainPattern(domainActInspect, `n\+1|slow quer\w*|consulta lenta|consultas? lentas|query plan|execution plan|plano de execu[cç][aã]o`),
		Weight:  2, Class: Medium,
	}},
	{Domain: "backend", ID: "be-consistency-design", SignalDef: SignalDef{
		Pattern: domainPattern(domainActPlan, `transactional outbox|outbox transacional|saga pattern|padr[aã]o saga|exactly[ -]?once|eventual consistency|consist[eê]ncia eventual|distributed transactions?|transa[cç][aã]o distribu[ií]da`),
		Weight:  3, Class: Complex,
	}},
	{Domain: "backend", ID: "be-api-contract-versioning", SignalDef: SignalDef{
		Pattern: domainPattern(domainActPlan, `api contract versioning|versionamento de contrato da api|versionamento de contratos?|backward compatibility|compatibilidade retroativa|openapi contract|contrato openapi`),
		Weight:  2, Class: Medium,
	}},
	{Domain: "fullstack", ID: "fs-api-ui-feature", SignalDef: SignalDef{
		Pattern: domainPattern(domainActBuild, `(?:ui|front[ -]?end|interface|tela).{0,100}(?:backend|back[ -]?end|api|database|banco de dados)`),
		Weight:  2, Class: Medium,
	}},
	{Domain: "fullstack", ID: "fs-end-to-end-feature", SignalDef: SignalDef{
		Pattern: domainPattern(domainActBuild, `(?:feature|funcionalidade).{0,65}(?:end[ -]?to[ -]?end|ponta a ponta|front[ -]?end.{0,30}back[ -]?end|api.{0,30}ui)`),
		Weight:  2, Class: Medium,
	}},
	{Domain: "fullstack", ID: "fs-coordinated-rollout", SignalDef: SignalDef{
		Pattern: domainPattern(domainActPlan, `(?:rollout|deploy|implanta[cç][aã]o|lan[cç]amento).{0,100}(?:schema|database|banco de dados).{0,100}(?:frontend|front[ -]?end|client|clientes?)`),
		Weight:  3, Class: Complex,
	}},
	{Domain: "fullstack", ID: "fs-contract-compatibility", SignalDef: SignalDef{
		Pattern: domainPattern(domainActBuild, `contract compatibility|compatibilidade de contratos?|compatibilidade de contrato|backward[ -]?compatible api|api retrocompat[ií]vel|client[ -]?server contract|contrato entre cliente e servidor`),
		Weight:  2, Class: Medium,
	}},
	{Domain: "devops", ID: "ops-crashloop-debug", SignalDef: SignalDef{
		Pattern: domainPattern(domainActInspect, `crashloopbackoff|oomkilled|pod eviction|evic[cç][aã]o de pods|pods? reiniciando|kubernetes scheduling|agendamento de pods`),
		Weight:  2, Class: Medium,
	}},
	{Domain: "devops", ID: "ops-helm-automation", SignalDef: SignalDef{
		Pattern: domainPattern(domainActAutomate, `helm chart|charts? helm|terraform module|m[oó]dulo terraform|kubernetes operator|operador kubernetes`),
		Weight:  2, Class: Medium,
	}},
	{Domain: "devops", ID: "ops-multi-region-recovery", SignalDef: SignalDef{
		Pattern: domainPattern(domainActPlan, `disaster recovery|recupera[cç][aã]o de desastres|multi[ -]?region failover|failover multirregional|active[ -]?active|ativo[ -]?ativo`),
		Weight:  3, Class: Complex,
	}},
	{Domain: "devops", ID: "ops-terraform-drift", SignalDef: SignalDef{
		Pattern: domainPattern(domainActInspect, `terraform drift|infrastructure drift|drift de infraestrutura|configuration drift|diverg[eê]ncia da infraestrutura`),
		Weight:  2, Class: Medium,
	}},
	{Domain: "finops", ID: "finops-anomaly-investigation", SignalDef: SignalDef{
		Pattern: domainPattern(domainActInspect, `cloud cost anomal\w*|anomalia de custos?|unexpected spend|gasto inesperado|egress cost|custo de egress|despesa de sa[ií]da de dados`),
		Weight:  2, Class: Medium,
	}},
	{Domain: "finops", ID: "finops-rightsizing-plan", SignalDef: SignalDef{
		Pattern: domainPattern(domainActOptimize, `cloud cost|custos? de nuvem|compute spend|gasto computacional|reserved instances|inst[aâ]ncias reservadas|savings plans?|planos? de economia`),
		Weight:  2, Class: Medium,
	}},
	{Domain: "finops", ID: "finops-chargeback-design", SignalDef: SignalDef{
		Pattern: domainPattern(domainActPlan, `multi[ -]?account chargeback|rateio de custos entre contas|showback model|modelo de showback|cost allocation across accounts|aloca[cç][aã]o de custos entre contas`),
		Weight:  3, Class: Complex,
	}},
	{Domain: "finops", ID: "finops-amortized-cost", SignalDef: SignalDef{
		Pattern: domainPattern(domainActInspect, `amortized cost|custo amortizado|effective cost|custo efetivo|commitment discount|desconto por compromisso|cost allocation|aloca[cç][aã]o de custos`),
		Weight:  2, Class: Medium,
	}},
	{Domain: "devsecops", ID: "sec-sast-triage", SignalDef: SignalDef{
		Pattern: domainPattern(domainActInspect, `sast|dast|dependency scanning|varredura de depend[eê]ncias|an[aá]lise est[aá]tica de seguran[cç]a|secret scanning|scan de segredos|an[aá]lise din[aâ]mica de seguran[cç]a`),
		Weight:  2, Class: Medium,
	}},
	{Domain: "devsecops", ID: "sec-supply-chain-hardening", SignalDef: SignalDef{
		Pattern: domainPattern(domainActProtect, `slsa|sbom|provenance attestations?|atestados? de proveni[eê]ncia|sigstore|cosign|software supply chain|cadeia de suprimentos de software`),
		Weight:  2, Class: Medium,
	}},
	{Domain: "devsecops", ID: "sec-threat-model-design", SignalDef: SignalDef{
		Pattern: domainPattern(domainActPlan, `threat model|modelo de amea[cç]as|zero trust architecture|arquitetura zero trust|trust boundar\w*|fronteiras? de confian[cç]a`),
		Weight:  3, Class: Complex,
	}},
	{Domain: "devsecops", ID: "sec-asvs-access-control", SignalDef: SignalDef{
		Pattern: domainPattern(domainActInspect, `broken access control|falha de controle de acesso|authorization bypass|bypass de autoriza[cç][aã]o|idor|privilege escalation|eleva[cç][aã]o de privil[eé]gios`),
		Weight:  2, Class: Medium,
	}},
	{Domain: "mobile", ID: "mob-deep-link-integration", SignalDef: SignalDef{
		Pattern: domainPattern(domainActBuild, `deep links?|universal links?|app links?|links? universais?|navega[cç][aã]o profunda|links? profundos?`),
		Weight:  2, Class: Medium,
	}},
	{Domain: "mobile", ID: "mob-anr-crash-debug", SignalDef: SignalDef{
		Pattern: domainPattern(domainActInspect, `android anr|anr|ios crash|app crash|crash no aplicativo|travamento no aplicativo|memory pressure|press[aã]o de mem[oó]ria`),
		Weight:  2, Class: Medium,
	}},
	{Domain: "mobile", ID: "mob-offline-conflict-design", SignalDef: SignalDef{
		Pattern: domainPattern(domainActPlan, `offline[ -]?first sync|offline sync conflict|conflict resolution offline|resolu[cç][aã]o de conflitos offline|sincroniza[cç][aã]o offline|cross[ -]?device state replication`),
		Weight:  3, Class: Complex,
	}},
	{Domain: "mobile", ID: "mob-ui-lifecycle-debug", SignalDef: SignalDef{
		Pattern: domainPattern(domainActInspect, `swiftui lifecycle|jetpack compose lifecycle|lifecycle.{0,35}(?:swiftui|jetpack compose)|ciclo de vida.{0,35}(?:swiftui|jetpack compose)|problema de ciclo de vida.{0,35}jetpack compose`),
		Weight:  2, Class: Medium,
	}},
	{Domain: "cicd", ID: "ci-flaky-test-repair", SignalDef: SignalDef{
		Pattern: domainPattern(domainActFix, `flaky tests?|testes? intermitentes?|pipeline flak\w*|instabilidade do pipeline|testes? inst[aá]veis`),
		Weight:  2, Class: Medium,
	}},
	{Domain: "cicd", ID: "ci-build-matrix", SignalDef: SignalDef{
		Pattern: domainPattern(domainActAutomate, `build matrix|matriz de builds?|test matrix|matriz de testes?|parallel jobs|jobs paralelos`),
		Weight:  2, Class: Medium,
	}},
	{Domain: "cicd", ID: "ci-progressive-delivery", SignalDef: SignalDef{
		Pattern: domainPattern(domainActPlan, `progressive delivery|canary rollout|blue[ -]?green deployment|automated rollback|rollback autom[aá]tico|deploy progressivo|entrega progressiva`),
		Weight:  3, Class: Complex,
	}},
	{Domain: "cicd", ID: "ci-cache-invalidation", SignalDef: SignalDef{
		Pattern: domainPattern(domainActFix, `pipeline cache invalidation|invalida[cç][aã]o de cache da pipeline|cache misses? no ci|falhas? de cache no ci|cache misses? in ci`),
		Weight:  2, Class: Medium,
	}},
}

// Compile once, with no runtime file parsing, network access or LLM call.
var enrichedSignals []signal

func init() {
	enrichedSignals = compileEnrichedSignals()
}

func compileEnrichedSignals() []signal {
	out := make([]signal, 0, len(signals)+len(RawDomainSignals))
	out = append(out, signals...)
	for _, s := range RawDomainSignals {
		out = append(out, signal{
			re:     regexp.MustCompile(`(?i)` + s.Pattern),
			weight: s.Weight,
			class:  s.Class,
		})
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
