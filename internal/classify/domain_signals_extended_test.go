// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package classify

import (
	"regexp"
	"testing"
)

// All extended rules must be unique across both Go tables and keep their
// complexity separate from model selection. Technical nouns alone are never
// allowed to choose a model.
func TestExtendedDomainSignalsIntegrity(t *testing.T) {
	ids := make(map[string]bool)
	domains := make(map[string]bool)
	for _, table := range [][]DomainSignalDef{RawDomainSignals, RawExtendedDomainSignals} {
		for _, rule := range table {
			if rule.ID == "" || rule.Domain == "" || ids[rule.ID] {
				t.Fatalf("duplicate or empty domain rule: %+v", rule)
			}
			ids[rule.ID] = true
			domains[rule.Domain] = true
			if rule.Weight < 1 || rule.Weight > 3 || (rule.Class != Medium && rule.Class != Complex) {
				t.Fatalf("%s: invalid weight or class", rule.ID)
			}
			if _, err := regexp.Compile("(?i)" + rule.Pattern); err != nil {
				t.Fatalf("%s: invalid regex: %v", rule.ID, err)
			}
		}
	}
	if len(domains) != 12 || len(ids) != 96 {
		t.Fatalf("expected 12 domains and 96 rules, got %d domains and %d rules", len(domains), len(ids))
	}
}

// Each new signal needs English, PT-BR, and code-switching examples.
// Regression cases check lexical match and avoid confident under-routing;
// broader corpus stability and cost checks run independently in internal/core.
func TestExtendedDomainSignalsBilingualParity(t *testing.T) {
	t.Setenv("DOWNSHIFT_DOMAIN_SIGNALS", "1")
	examples := []struct{ id, en, pt, mixed string }{
		{"fe-keyboard-a11y", "Test keyboard navigation", "Teste navegação por teclado", "Verifique focus management"},
		{"fe-bundle-size", "Optimize bundle size", "Otimize tamanho do bundle", "Ajuste JavaScript payload"},
		{"fe-state-sync", "Investigate state synchronization", "Investigue sincronização de estado", "Corrija state update loops"},
		{"fe-form-validation", "Implement client-side form validation", "Implemente validação de formulário", "Crie form validation"},
		{"fe-ssr-streaming", "Design SSR streaming architecture", "Projete arquitetura de streaming SSR", "Planeje SSR streaming architecture"},
		{"fe-visual-regression", "Test visual regression tests", "Teste testes de regressão visual", "Verifique screenshot diff"},
		{"be-dlq-debug", "Investigate dead-letter queue", "Investigue fila de mensagens mortas", "Corrija DLQ backlog"},
		{"be-cache-stampede", "Investigate cache stampede", "Investigue efeito manada no cache", "Corrija cache thundering herd"},
		{"be-idempotency-keys", "Implement idempotency key validation", "Implemente validação de chave de idempotência", "Crie idempotency middleware"},
		{"be-websocket-reconnect", "Implement WebSocket reconnect logic", "Implemente lógica de reconexão websocket", "Crie WebSocket backoff"},
		{"be-transaction-boundary", "Design transaction boundary across services", "Projete fronteira transacional entre serviços", "Planeje multi-service transaction strategy"},
		{"be-batch-throughput", "Optimize batch processing", "Otimize processamento em lote", "Ajuste batch throughput"},
		{"fs-session-flow", "Implement end-to-end session flow", "Implemente fluxo de sessão ponta a ponta", "Crie client-server session lifecycle"},
		{"fs-upload-progress", "Implement upload progress UI and API", "Implemente upload com progresso UI e API", "Crie upload progress across client and server"},
		{"fs-realtime-clients", "Design realtime synchronization across clients", "Projete sincronização em tempo real entre clientes", "Planeje cross-client realtime state"},
		{"fs-internationalization", "Implement internationalization across frontend and backend", "Implemente internacionalização entre frontend e backend", "Crie shared localization between UI and API"},
		{"fs-api-ui-errors", "Implement API error handling in UI", "Implemente tratamento de erros de API na interface", "Crie client-server error states"},
		{"fs-tenant-isolation", "Design tenant isolation across frontend and backend", "Projete isolamento de tenant entre frontend e backend", "Planeje multi-tenant access boundaries"},
		{"ops-hpa-autoscale", "Automate horizontal pod autoscaler", "Automatize escalonamento horizontal de pods", "Configure HPA configuration"},
		{"ops-dns-discovery", "Investigate service discovery DNS", "Investigue descoberta de serviços DNS", "Corrija Kubernetes DNS resolution"},
		{"ops-container-permissions", "Harden container image permissions", "Implemente permissões de imagem de contêiner", "Configure rootless container setup"},
		{"ops-config-reload", "Automate ConfigMap reload", "Automatize recarga de ConfigMap", "Configure configuration reload across pods"},
		{"ops-volume-expansion", "Design persistent volume expansion without downtime", "Projete expansão de volume persistente sem downtime", "Planeje stateful storage migration across clusters"},
		{"ops-service-mesh-mtls", "Design service mesh mTLS rollout", "Projete implantação mTLS da malha de serviços", "Planeje cross-cluster service mesh identity"},
		{"finops-cost-tags", "Implement cost allocation tags", "Implemente etiquetas de alocação de custos", "Crie tagging for cloud chargeback"},
		{"finops-spot-usage", "Optimize spot instance utilization", "Otimize uso de instâncias spot", "Ajuste preemptible workload costs"},
		{"finops-idle-resources", "Investigate idle cloud resources", "Investigue recursos ociosos em nuvem", "Corrija unused provisioned compute"},
		{"finops-budget-anomalies", "Implement budget anomaly alerts", "Implemente alertas de anomalia orçamentária", "Crie budget threshold notification"},
		{"finops-commitment-portfolio", "Design reserved instance commitment portfolio", "Projete portfólio de compromissos de instâncias reservadas", "Planeje multi-provider savings commitments"},
		{"finops-cross-region-egress", "Optimize cross-region data transfer costs", "Otimize custos de transferência de dados entre regiões", "Ajuste inter-region egress"},
		{"sec-csp-policy", "Harden Content Security Policy", "Implemente política de segurança de conteúdo", "Configure CSP rollout"},
		{"sec-dependency-cve", "Investigate vulnerable dependency CVE", "Investigue dependência vulnerável CVE", "Corrija dependency advisory"},
		{"sec-workload-identity", "Harden workload identity federation", "Implemente federação de identidade de workloads", "Configure OIDC workload identity"},
		{"sec-sbom-license", "Test SBOM license compliance", "Teste conformidade de licenças SBOM", "Verifique software license policy"},
		{"sec-cross-cluster-rbac", "Design Kubernetes RBAC migration across clusters", "Projete migração de RBAC Kubernetes entre clusters", "Planeje cross-cluster permissions model"},
		{"sec-key-rotation-policy", "Harden key rotation policy", "Implemente política de rotação de chaves", "Configure service credential lifecycle"},
		{"mob-push-delivery", "Implement push notification delivery", "Implemente entrega de notificações push", "Crie push notification retries"},
		{"mob-cold-start", "Optimize app cold start", "Otimize inicialização a frio do aplicativo", "Ajuste mobile app startup time"},
		{"mob-background-location", "Implement background location permissions", "Implemente permissões de localização em segundo plano", "Crie background permission prompt"},
		{"mob-testflight-release", "Automate TestFlight rollout", "Automatize implantação via TestFlight", "Configure Play Console staged release"},
		{"mob-offline-db-migration", "Design offline database schema migration", "Projete migração de schema offline do banco local", "Planeje on-device data migration"},
		{"mob-battery-background", "Optimize background battery consumption", "Otimize consumo de bateria em segundo plano", "Ajuste background wakelock usage"},
		{"ci-actions-permissions", "Harden GitHub Actions permissions", "Implemente permissões de GitHub Actions", "Configure CI workflow token scope"},
		{"ci-artifact-provenance", "Test build artifact provenance", "Teste proveniência de artefatos de build", "Verifique artifact attestations"},
		{"ci-monorepo-cache", "Optimize monorepo cache", "Otimize cache de monorepo", "Ajuste affected build cache"},
		{"ci-semver-release", "Automate semantic version release", "Automatize release com versionamento semântico", "Configure semver changelog"},
		{"ci-affected-matrix", "Implement affected packages matrix", "Implemente matriz de pacotes afetados", "Crie monorepo affected jobs"},
		{"ci-secret-exposure", "Investigate pipeline secret exposure", "Investigue exposição de segredos na pipeline", "Corrija CI leaked credentials"},
		{"qa-consumer-contract", "Test consumer-driven contract tests", "Teste testes de contrato orientados ao consumidor", "Verifique contract test compatibility"},
		{"qa-property-based", "Implement property-based tests", "Implemente testes baseados em propriedades", "Crie generative test cases"},
		{"qa-load-p99", "Test load testing latency p99", "Teste teste de carga latência p99", "Verifique performance test p99"},
		{"qa-e2e-flakiness", "Investigate E2E flakiness", "Investigue instabilidade dos testes ponta a ponta", "Corrija end-to-end flaky suite"},
		{"data-cdc-multi-db", "Design CDC across multiple databases", "Projete CDC entre múltiplos bancos de dados", "Planeje change data capture across services"},
		{"data-online-backfill", "Design large table backfill without downtime", "Projete preenchimento retroativo de tabela sem downtime", "Planeje online historical data backfill"},
		{"data-quality-constraints", "Test data quality constraints", "Teste restrições de qualidade de dados", "Verifique data freshness checks"},
		{"data-stream-consumer-lag", "Investigate stream consumer lag", "Investigue atraso de consumidor de streaming", "Corrija Kafka consumer lag"},
		{"ai-rag-retrieval", "Optimize RAG retrieval recall", "Otimize recall da recuperação RAG", "Ajuste retrieval reranking quality"},
		{"ai-agent-tool-errors", "Investigate agent tool calling failures", "Investigue falhas de chamadas de ferramentas dos agentes", "Corrija agent tool schema mismatch"},
		{"ai-eval-regression-set", "Implement regression eval dataset", "Implemente conjunto de evals de regressão", "Crie LLM evaluation fixtures"},
		{"ai-multi-agent-hitl", "Design multi-agent orchestration with HITL", "Projete orquestração multiagente com HITL", "Planeje human-in-the-loop agent workflow"},
		{"obs-trace-correlation", "Implement distributed trace correlation", "Implemente correlação de traces distribuídos", "Crie trace-log correlation"},
		{"obs-cross-service-slo", "Design cross-service SLO error budget policy", "Projete política de orçamento de erro SLO entre serviços", "Planeje cross-service reliability objectives"},
		{"obs-metric-cardinality", "Investigate metric label cardinality", "Investigue cardinalidade de labels das métricas", "Corrija high cardinality metrics"},
		{"obs-alert-fatigue", "Optimize alert noise reduction", "Otimize redução de ruído de alertas", "Ajuste alert fatigue"},
	}
	if len(examples) != len(RawExtendedDomainSignals) {
		t.Fatalf("expected one bilingual group per new rule, got %d / %d", len(examples), len(RawExtendedDomainSignals))
	}
	byID := make(map[string]DomainSignalDef)
	for _, rule := range RawExtendedDomainSignals {
		byID[rule.ID] = rule
	}
	for _, pair := range examples {
		t.Run(pair.id, func(t *testing.T) {
			rule, ok := byID[pair.id]
			if !ok {
				t.Fatalf("unknown rule %s", pair.id)
			}
			re := regexp.MustCompile("(?i)" + rule.Pattern)
			for _, phrase := range []struct{ lang, text string }{
				{"en", pair.en}, {"pt-BR", pair.pt}, {"mixed", pair.mixed},
			} {
				if !re.MatchString(phrase.text) {
					t.Errorf("%s failed to match %q", phrase.lang, phrase.text)
				}
				classification := Classify(phrase.text)
				if classification.Complexity < Medium {
					t.Errorf("%s incorrectly routed below mid for %q: %s", phrase.lang, phrase.text, classification.Complexity)
				}
			}
		})
	}
}

// No rule may activate on technology names or informational prompts alone.
// Add adversarial negatives when new domain vocabulary is proposed.
func TestExtendedDomainSignalsNegativeControls(t *testing.T) {
	prompts := []string{
		"React", "Kubernetes", "Terraform", "SAST", "HITL", "Kafka",
		"TypeScript", "SBOM", "FinOps", "TestFlight",
		"Explain what a dead-letter queue is",
		"Explique o que é cardinalidade de métricas",
		"Summarize the guide to RAG retrieval",
		"Liste os arquivos que usam WebSocket",
		"Read the design doc about service mesh mTLS",
	}
	for _, phrase := range prompts {
		for _, rule := range RawExtendedDomainSignals {
			if regexp.MustCompile("(?i)" + rule.Pattern).MatchString(phrase) {
				t.Errorf("%q unexpectedly triggered %s", phrase, rule.ID)
			}
		}
	}
}
