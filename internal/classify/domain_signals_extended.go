// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package classify

// Domain expansion stays in a separate Go table: review small groups by domain,
// require bilingual equivalents, and tune weights using reviewed misroutes.
// No domain name is itself a score, and model choice remains in core policy.
//
// DomainActVerify is intentionally more focused than the general "implement"
// verbs. It recognizes validation/testing requests in English and Portuguese.
const domainActVerify = `(?:test\w*|verific\w*|verifiq\w*|validat\w*|valid\w*|check|chec\w*|assert\w*|confir\w*)`

// RawExtendedDomainSignals adds high-context developer tasks without creating
// a second classifier or adding a runtime parser, network call, or LLM call.
// Every row needs English, Portuguese, and mixed-language regression examples.
// Keep complex signals narrow to avoid needless frontier escalations.
var RawExtendedDomainSignals = []DomainSignalDef{
	{Domain: "frontend", ID: "fe-keyboard-a11y", SignalDef: SignalDef{
		Pattern: domainPattern(domainActVerify, `keyboard navigation|navega[cç][aã]o por teclado|focus management|gerenciamento de foco`),
		Weight:  2, Class: Medium,
	}},
	{Domain: "frontend", ID: "fe-bundle-size", SignalDef: SignalDef{
		Pattern: domainPattern(domainActOptimize, `bundle size|tamanho do bundle|javascript payload|peso do javascript`),
		Weight:  2, Class: Medium,
	}},
	{Domain: "frontend", ID: "fe-state-sync", SignalDef: SignalDef{
		Pattern: domainPattern(domainActInspect, `state synchronization|sincroniza[cç][aã]o de estado|state update loops?|loops? de atualiza[cç][aã]o de estado`),
		Weight:  2, Class: Medium,
	}},
	{Domain: "frontend", ID: "fe-form-validation", SignalDef: SignalDef{
		Pattern: domainPattern(domainActBuild, `client[ -]?side form validation|valida[cç][aã]o de formul[aá]rios?|form validation|valida[cç][aã]o de campos`),
		Weight:  2, Class: Medium,
	}},
	{Domain: "frontend", ID: "fe-ssr-streaming", SignalDef: SignalDef{
		Pattern: domainPattern(domainActPlan, `ssr streaming architecture|arquitetura de streaming ssr|multi[ -]?route server rendering|renderiza[cç][aã]o de m[uú]ltiplas rotas no servidor`),
		Weight:  3, Class: Complex,
	}},
	{Domain: "frontend", ID: "fe-visual-regression", SignalDef: SignalDef{
		Pattern: domainPattern(domainActVerify, `visual regression tests?|testes? de regress[aã]o visual|screenshot diff|diferen[cç]a de screenshots?`),
		Weight:  2, Class: Medium,
	}},
	{Domain: "backend", ID: "be-dlq-debug", SignalDef: SignalDef{
		Pattern: domainPattern(domainActInspect, `dead[ -]?letter queue|fila de mensagens mortas|dlq backlog|ac[uú]mulo na dlq`),
		Weight:  2, Class: Medium,
	}},
	{Domain: "backend", ID: "be-cache-stampede", SignalDef: SignalDef{
		Pattern: domainPattern(domainActInspect, `cache stampede|efeito manada no cache|cache thundering herd|sobrecarga simult[aâ]nea do cache`),
		Weight:  2, Class: Medium,
	}},
	{Domain: "backend", ID: "be-idempotency-keys", SignalDef: SignalDef{
		Pattern: domainPattern(domainActBuild, `idempotency key validation|valida[cç][aã]o de chaves? de idempot[eê]ncia|idempotency middleware|middleware de idempot[eê]ncia`),
		Weight:  2, Class: Medium,
	}},
	{Domain: "backend", ID: "be-websocket-reconnect", SignalDef: SignalDef{
		Pattern: domainPattern(domainActBuild, `websocket reconnect logic|l[oó]gica de reconex[aã]o websocket|websocket backoff|reconex[aã]o exponencial websocket`),
		Weight:  2, Class: Medium,
	}},
	{Domain: "backend", ID: "be-transaction-boundary", SignalDef: SignalDef{
		Pattern: domainPattern(domainActPlan, `transaction boundary across services|fronteira transacional entre servi[cç]os|multi[ -]?service transaction strategy|estrat[eé]gia transacional entre servi[cç]os`),
		Weight:  3, Class: Complex,
	}},
	{Domain: "backend", ID: "be-batch-throughput", SignalDef: SignalDef{
		Pattern: domainPattern(domainActOptimize, `batch processing|processamento em lote|batch throughput|vaz[aã]o de lotes`),
		Weight:  2, Class: Medium,
	}},
	{Domain: "fullstack", ID: "fs-session-flow", SignalDef: SignalDef{
		Pattern: domainPattern(domainActBuild, `end[ -]?to[ -]?end session flow|fluxo de sess[aã]o ponta a ponta|client[ -]?server session lifecycle|ciclo de sess[aã]o entre cliente e servidor`),
		Weight:  2, Class: Medium,
	}},
	{Domain: "fullstack", ID: "fs-upload-progress", SignalDef: SignalDef{
		Pattern: domainPattern(domainActBuild, `upload progress ui and api|upload com progresso ui e api|upload progress across client and server|progresso de upload entre cliente e servidor`),
		Weight:  2, Class: Medium,
	}},
	{Domain: "fullstack", ID: "fs-realtime-clients", SignalDef: SignalDef{
		Pattern: domainPattern(domainActPlan, `realtime synchronization across clients|sincroniza[cç][aã]o em tempo real entre clientes|cross[ -]?client realtime state|estado em tempo real entre clientes`),
		Weight:  3, Class: Complex,
	}},
	{Domain: "fullstack", ID: "fs-internationalization", SignalDef: SignalDef{
		Pattern: domainPattern(domainActBuild, `internationalization across frontend and backend|internacionaliza[cç][aã]o entre frontend e backend|shared localization between ui and api|localiza[cç][aã]o compartilhada entre ui e api`),
		Weight:  2, Class: Medium,
	}},
	{Domain: "fullstack", ID: "fs-api-ui-errors", SignalDef: SignalDef{
		Pattern: domainPattern(domainActBuild, `api error handling in ui|tratamento de erros de api na interface|client[ -]?server error states|estados de erro entre cliente e servidor`),
		Weight:  2, Class: Medium,
	}},
	{Domain: "fullstack", ID: "fs-tenant-isolation", SignalDef: SignalDef{
		Pattern: domainPattern(domainActPlan, `tenant isolation across frontend and backend|isolamento de tenants? entre frontend e backend|multi[ -]?tenant access boundaries|fronteiras de acesso multi[ -]?tenant`),
		Weight:  3, Class: Complex,
	}},
	{Domain: "devops", ID: "ops-hpa-autoscale", SignalDef: SignalDef{
		Pattern: domainPattern(domainActAutomate, `horizontal pod autoscaler|escalonamento horizontal de pods|hpa configuration|configura[cç][aã]o de hpa`),
		Weight:  2, Class: Medium,
	}},
	{Domain: "devops", ID: "ops-dns-discovery", SignalDef: SignalDef{
		Pattern: domainPattern(domainActInspect, `service discovery dns|descoberta de servi[cç]os dns|kubernetes dns resolution|resolu[cç][aã]o dns no kubernetes`),
		Weight:  2, Class: Medium,
	}},
	{Domain: "devops", ID: "ops-container-permissions", SignalDef: SignalDef{
		Pattern: domainPattern(domainActProtect, `container image permissions|permiss[oõ]es de imagem de cont[eê]iner|rootless container setup|configura[cç][aã]o de cont[eê]iner sem root`),
		Weight:  2, Class: Medium,
	}},
	{Domain: "devops", ID: "ops-config-reload", SignalDef: SignalDef{
		Pattern: domainPattern(domainActAutomate, `configmap reload|recarga de configmap|configuration reload across pods|recarga de configura[cç][aã]o entre pods`),
		Weight:  2, Class: Medium,
	}},
	{Domain: "devops", ID: "ops-volume-expansion", SignalDef: SignalDef{
		Pattern: domainPattern(domainActPlan, `persistent volume expansion without downtime|expans[aã]o de volume persistente sem downtime|stateful storage migration across clusters|migra[cç][aã]o de armazenamento entre clusters`),
		Weight:  3, Class: Complex,
	}},
	{Domain: "devops", ID: "ops-service-mesh-mtls", SignalDef: SignalDef{
		Pattern: domainPattern(domainActPlan, `service mesh mtls rollout|implanta[cç][aã]o mtls da malha de servi[cç]os|cross[ -]?cluster service mesh identity|identidade de malha de servi[cç]os entre clusters`),
		Weight:  3, Class: Complex,
	}},
	{Domain: "finops", ID: "finops-cost-tags", SignalDef: SignalDef{
		Pattern: domainPattern(domainActBuild, `cost allocation tags|etiquetas de aloca[cç][aã]o de custos|tagging for cloud chargeback|tags para rateio de custos em nuvem`),
		Weight:  2, Class: Medium,
	}},
	{Domain: "finops", ID: "finops-spot-usage", SignalDef: SignalDef{
		Pattern: domainPattern(domainActOptimize, `spot instance utilization|uso de inst[aâ]ncias spot|preemptible workload costs|custos de cargas preemptivas`),
		Weight:  2, Class: Medium,
	}},
	{Domain: "finops", ID: "finops-idle-resources", SignalDef: SignalDef{
		Pattern: domainPattern(domainActInspect, `idle cloud resources|recursos ociosos em nuvem|unused provisioned compute|computa[cç][aã]o provisionada sem uso`),
		Weight:  2, Class: Medium,
	}},
	{Domain: "finops", ID: "finops-budget-anomalies", SignalDef: SignalDef{
		Pattern: domainPattern(domainActBuild, `budget anomaly alerts|alertas de anomalia or[cç]ament[aá]ria|budget threshold notification|notifica[cç][aã]o de limite de or[cç]amento`),
		Weight:  2, Class: Medium,
	}},
	{Domain: "finops", ID: "finops-commitment-portfolio", SignalDef: SignalDef{
		Pattern: domainPattern(domainActPlan, `reserved instance commitment portfolio|portf[oó]lio de compromissos de inst[aâ]ncias reservadas|multi[ -]?provider savings commitments|compromissos de economia entre provedores`),
		Weight:  2, Class: Medium,
	}},
	{Domain: "finops", ID: "finops-cross-region-egress", SignalDef: SignalDef{
		Pattern: domainPattern(domainActOptimize, `cross[ -]?region data transfer costs|custos de transfer[eê]ncia de dados entre regi[oõ]es|inter[ -]?region egress|egress entre regi[oõ]es`),
		Weight:  2, Class: Medium,
	}},
	{Domain: "devsecops", ID: "sec-csp-policy", SignalDef: SignalDef{
		Pattern: domainPattern(domainActProtect, `content security policy|pol[ií]tica de seguran[cç]a de conte[uú]do|csp rollout|implanta[cç][aã]o de csp`),
		Weight:  2, Class: Medium,
	}},
	{Domain: "devsecops", ID: "sec-dependency-cve", SignalDef: SignalDef{
		Pattern: domainPattern(domainActInspect, `vulnerable dependenc(?:y|ies) cve|depend[eê]ncia vulner[aá]vel cve|dependency advisory|aviso de seguran[cç]a de depend[eê]ncia`),
		Weight:  2, Class: Medium,
	}},
	{Domain: "devsecops", ID: "sec-workload-identity", SignalDef: SignalDef{
		Pattern: domainPattern(domainActProtect, `workload identity federation|federa[cç][aã]o de identidade de workloads?|oidc workload identity|identidade de workload oidc`),
		Weight:  2, Class: Medium,
	}},
	{Domain: "devsecops", ID: "sec-sbom-license", SignalDef: SignalDef{
		Pattern: domainPattern(domainActVerify, `sbom license compliance|conformidade de licen[cç]as sbom|software license policy|pol[ií]tica de licen[cç]as de software`),
		Weight:  2, Class: Medium,
	}},
	{Domain: "devsecops", ID: "sec-cross-cluster-rbac", SignalDef: SignalDef{
		Pattern: domainPattern(domainActPlan, `kubernetes rbac migration across clusters|migra[cç][aã]o de rbac kubernetes entre clusters|cross[ -]?cluster permissions model|modelo de permiss[oõ]es entre clusters`),
		Weight:  3, Class: Complex,
	}},
	{Domain: "devsecops", ID: "sec-key-rotation-policy", SignalDef: SignalDef{
		Pattern: domainPattern(domainActProtect, `key rotation policy|pol[ií]tica de rota[cç][aã]o de chaves|service credential lifecycle|ciclo de credenciais de servi[cç]o`),
		Weight:  2, Class: Medium,
	}},
	{Domain: "mobile", ID: "mob-push-delivery", SignalDef: SignalDef{
		Pattern: domainPattern(domainActBuild, `push notification delivery|entrega de notifica[cç][oõ]es push|push notification retries|retentativas de notifica[cç][oõ]es push`),
		Weight:  2, Class: Medium,
	}},
	{Domain: "mobile", ID: "mob-cold-start", SignalDef: SignalDef{
		Pattern: domainPattern(domainActOptimize, `app cold start|inicializa[cç][aã]o a frio do aplicativo|mobile app startup time|tempo de abertura do aplicativo`),
		Weight:  2, Class: Medium,
	}},
	{Domain: "mobile", ID: "mob-background-location", SignalDef: SignalDef{
		Pattern: domainPattern(domainActBuild, `background location permissions|permiss[oõ]es de localiza[cç][aã]o em segundo plano|background permission prompt|solicita[cç][aã]o de permiss[aã]o em segundo plano`),
		Weight:  2, Class: Medium,
	}},
	{Domain: "mobile", ID: "mob-testflight-release", SignalDef: SignalDef{
		Pattern: domainPattern(domainActAutomate, `testflight rollout|implanta[cç][aã]o via testflight|play console staged release|lan[cç]amento gradual no play console`),
		Weight:  2, Class: Medium,
	}},
	{Domain: "mobile", ID: "mob-offline-db-migration", SignalDef: SignalDef{
		Pattern: domainPattern(domainActPlan, `offline database schema migration|migra[cç][aã]o de schema offline do banco local|on[ -]?device data migration|migra[cç][aã]o de dados no dispositivo`),
		Weight:  3, Class: Complex,
	}},
	{Domain: "mobile", ID: "mob-battery-background", SignalDef: SignalDef{
		Pattern: domainPattern(domainActOptimize, `background battery consumption|consumo de bateria em segundo plano|background wakelock usage|uso de wakelock em segundo plano`),
		Weight:  2, Class: Medium,
	}},
	{Domain: "cicd", ID: "ci-actions-permissions", SignalDef: SignalDef{
		Pattern: domainPattern(domainActProtect, `github actions permissions|permiss[oõ]es de github actions|ci workflow token scope|escopo do token de workflow ci`),
		Weight:  2, Class: Medium,
	}},
	{Domain: "cicd", ID: "ci-artifact-provenance", SignalDef: SignalDef{
		Pattern: domainPattern(domainActVerify, `build artifact provenance|proveni[eê]ncia de artefatos de build|artifact attestations|atesta[cç][oõ]es de artefatos`),
		Weight:  2, Class: Medium,
	}},
	{Domain: "cicd", ID: "ci-monorepo-cache", SignalDef: SignalDef{
		Pattern: domainPattern(domainActOptimize, `monorepo cache|cache de monorepo|affected build cache|cache de builds afetados`),
		Weight:  2, Class: Medium,
	}},
	{Domain: "cicd", ID: "ci-semver-release", SignalDef: SignalDef{
		Pattern: domainPattern(domainActAutomate, `semantic version release|release com versionamento sem[aâ]ntico|semver changelog|changelog de vers[aã]o sem[aâ]ntica`),
		Weight:  2, Class: Medium,
	}},
	{Domain: "cicd", ID: "ci-affected-matrix", SignalDef: SignalDef{
		Pattern: domainPattern(domainActBuild, `affected packages? matrix|matriz de pacotes afetados|monorepo affected jobs|jobs afetados no monorepo`),
		Weight:  2, Class: Medium,
	}},
	{Domain: "cicd", ID: "ci-secret-exposure", SignalDef: SignalDef{
		Pattern: domainPattern(domainActInspect, `pipeline secret exposure|exposi[cç][aã]o de segredos na pipeline|ci leaked credentials|credenciais vazadas no ci`),
		Weight:  2, Class: Medium,
	}},
	{Domain: "testing", ID: "qa-consumer-contract", SignalDef: SignalDef{
		Pattern: domainPattern(domainActVerify, `consumer[ -]?driven contract tests|testes de contrato orientados ao consumidor|contract test compatibility|compatibilidade de testes de contrato`),
		Weight:  2, Class: Medium,
	}},
	{Domain: "testing", ID: "qa-property-based", SignalDef: SignalDef{
		Pattern: domainPattern(domainActBuild, `property[ -]?based tests|testes baseados em propriedades|generative test cases|casos de teste generativos`),
		Weight:  2, Class: Medium,
	}},
	{Domain: "testing", ID: "qa-load-p99", SignalDef: SignalDef{
		Pattern: domainPattern(domainActVerify, `load test(?:ing)? latency p99|teste de carga lat[eê]ncia p99|performance test p99|teste de desempenho p99`),
		Weight:  2, Class: Medium,
	}},
	{Domain: "testing", ID: "qa-e2e-flakiness", SignalDef: SignalDef{
		Pattern: domainPattern(domainActInspect, `e2e flakiness|instabilidade dos testes ponta a ponta|end[ -]?to[ -]?end flaky suite|su[ií]te e2e inst[aá]vel`),
		Weight:  2, Class: Medium,
	}},
	{Domain: "data", ID: "data-cdc-multi-db", SignalDef: SignalDef{
		Pattern: domainPattern(domainActPlan, `cdc across multiple databases|cdc entre m[uú]ltiplos bancos de dados|change data capture across services|captura de altera[cç][oõ]es entre servi[cç]os`),
		Weight:  3, Class: Complex,
	}},
	{Domain: "data", ID: "data-online-backfill", SignalDef: SignalDef{
		Pattern: domainPattern(domainActPlan, `large table backfill without downtime|preenchimento retroativo de tabela sem downtime|online historical data backfill|carga retroativa de dados sem parada`),
		Weight:  3, Class: Complex,
	}},
	{Domain: "data", ID: "data-quality-constraints", SignalDef: SignalDef{
		Pattern: domainPattern(domainActVerify, `data quality constraints|restri[cç][oõ]es de qualidade de dados|data freshness checks|verifica[cç][oõ]es de atualidade de dados`),
		Weight:  2, Class: Medium,
	}},
	{Domain: "data", ID: "data-stream-consumer-lag", SignalDef: SignalDef{
		Pattern: domainPattern(domainActInspect, `stream consumer lag|atraso de consumidor de streaming|kafka consumer lag|atraso de consumidor kafka`),
		Weight:  2, Class: Medium,
	}},
	{Domain: "ai", ID: "ai-rag-retrieval", SignalDef: SignalDef{
		Pattern: domainPattern(domainActOptimize, `rag retrieval recall|recall da recupera[cç][aã]o rag|retrieval reranking quality|qualidade de reranking da busca`),
		Weight:  2, Class: Medium,
	}},
	{Domain: "ai", ID: "ai-agent-tool-errors", SignalDef: SignalDef{
		Pattern: domainPattern(domainActInspect, `agent tool calling failures|falhas de chamadas de ferramentas dos agentes|agent tool schema mismatch|incompatibilidade de schema de ferramenta do agente`),
		Weight:  2, Class: Medium,
	}},
	{Domain: "ai", ID: "ai-eval-regression-set", SignalDef: SignalDef{
		Pattern: domainPattern(domainActBuild, `regression eval dataset|conjunto de evals de regress[aã]o|llm evaluation fixtures|casos de avalia[cç][aã]o de llm`),
		Weight:  2, Class: Medium,
	}},
	{Domain: "ai", ID: "ai-multi-agent-hitl", SignalDef: SignalDef{
		Pattern: domainPattern(domainActPlan, `multi[ -]?agent orchestration with hitl|orquestra[cç][aã]o multiagente com hitl|human[ -]?in[ -]?the[ -]?loop agent workflow|fluxo multiagente com aprova[cç][aã]o humana`),
		Weight:  3, Class: Complex,
	}},
	{Domain: "observability", ID: "obs-trace-correlation", SignalDef: SignalDef{
		Pattern: domainPattern(domainActBuild, `distributed trace correlation|correla[cç][aã]o de traces distribu[ií]dos|trace[ -]?log correlation|correla[cç][aã]o de logs e traces`),
		Weight:  2, Class: Medium,
	}},
	{Domain: "observability", ID: "obs-cross-service-slo", SignalDef: SignalDef{
		Pattern: domainPattern(domainActPlan, `cross[ -]?service slo error budget policy|pol[ií]tica de or[cç]amento de erro slo entre servi[cç]os|cross[ -]?service reliability objectives|metas de confiabilidade entre servi[cç]os`),
		Weight:  3, Class: Complex,
	}},
	{Domain: "observability", ID: "obs-metric-cardinality", SignalDef: SignalDef{
		Pattern: domainPattern(domainActInspect, `metric label cardinality|cardinalidade de labels das m[eé]tricas|high cardinality metrics|m[eé]tricas de alta cardinalidade`),
		Weight:  2, Class: Medium,
	}},
	{Domain: "observability", ID: "obs-alert-fatigue", SignalDef: SignalDef{
		Pattern: domainPattern(domainActOptimize, `alert noise reduction|redu[cç][aã]o de ru[ií]do de alertas|alert fatigue|fadiga de alertas`),
		Weight:  2, Class: Medium,
	}},
}
