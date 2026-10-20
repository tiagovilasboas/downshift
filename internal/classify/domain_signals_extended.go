// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package classify

// Domain expansion stays in a separate Go table: review small groups by domain,
// require bilingual equivalents, and tune weights using reviewed misroutes.
// No domain name is itself a score, and model choice remains in core policy.
//
// RawExtendedDomainSignals adds high-context developer tasks without creating
// a second classifier or adding a runtime parser, network call, or LLM call.
// Every row needs English, Portuguese, and mixed-language regression examples.
// Keep complex signals narrow to avoid needless frontier escalations.
var RawExtendedDomainSignals = []DomainSignalDef{
	{Domain: "frontend", ID: "fe-keyboard-a11y", SignalDef: SignalDef{
		Pattern: domainPatternFromLexicon("fe-keyboard-a11y", "verify"),
		Weight:  2, Class: Medium,
	}},
	{Domain: "frontend", ID: "fe-bundle-size", SignalDef: SignalDef{
		Pattern: domainPatternFromLexicon("fe-bundle-size", "optimize"),
		Weight:  2, Class: Medium,
	}},
	{Domain: "frontend", ID: "fe-state-sync", SignalDef: SignalDef{
		Pattern: domainPatternFromLexicon("fe-state-sync", "inspect"),
		Weight:  2, Class: Medium,
	}},
	{Domain: "frontend", ID: "fe-form-validation", SignalDef: SignalDef{
		Pattern: domainPatternFromLexicon("fe-form-validation", "build"),
		Weight:  2, Class: Medium,
	}},
	{Domain: "frontend", ID: "fe-ssr-streaming", SignalDef: SignalDef{
		Pattern: domainPatternFromLexicon("fe-ssr-streaming", "plan"),
		Weight:  3, Class: Complex,
	}},
	{Domain: "frontend", ID: "fe-visual-regression", SignalDef: SignalDef{
		Pattern: domainPatternFromLexicon("fe-visual-regression", "verify"),
		Weight:  2, Class: Medium,
	}},
	{Domain: "backend", ID: "be-dlq-debug", SignalDef: SignalDef{
		Pattern: domainPatternFromLexicon("be-dlq-debug", "inspect"),
		Weight:  2, Class: Medium,
	}},
	{Domain: "backend", ID: "be-cache-stampede", SignalDef: SignalDef{
		Pattern: domainPatternFromLexicon("be-cache-stampede", "inspect"),
		Weight:  2, Class: Medium,
	}},
	{Domain: "backend", ID: "be-idempotency-keys", SignalDef: SignalDef{
		Pattern: domainPatternFromLexicon("be-idempotency-keys", "build"),
		Weight:  2, Class: Medium,
	}},
	{Domain: "backend", ID: "be-websocket-reconnect", SignalDef: SignalDef{
		Pattern: domainPatternFromLexicon("be-websocket-reconnect", "build"),
		Weight:  2, Class: Medium,
	}},
	{Domain: "backend", ID: "be-transaction-boundary", SignalDef: SignalDef{
		Pattern: domainPatternFromLexicon("be-transaction-boundary", "plan"),
		Weight:  3, Class: Complex,
	}},
	{Domain: "backend", ID: "be-batch-throughput", SignalDef: SignalDef{
		Pattern: domainPatternFromLexicon("be-batch-throughput", "optimize"),
		Weight:  2, Class: Medium,
	}},
	{Domain: "fullstack", ID: "fs-session-flow", SignalDef: SignalDef{
		Pattern: domainPatternFromLexicon("fs-session-flow", "build"),
		Weight:  2, Class: Medium,
	}},
	{Domain: "fullstack", ID: "fs-upload-progress", SignalDef: SignalDef{
		Pattern: domainPatternFromLexicon("fs-upload-progress", "build"),
		Weight:  2, Class: Medium,
	}},
	{Domain: "fullstack", ID: "fs-realtime-clients", SignalDef: SignalDef{
		Pattern: domainPatternFromLexicon("fs-realtime-clients", "plan"),
		Weight:  3, Class: Complex,
	}},
	{Domain: "fullstack", ID: "fs-internationalization", SignalDef: SignalDef{
		Pattern: domainPatternFromLexicon("fs-internationalization", "build"),
		Weight:  2, Class: Medium,
	}},
	{Domain: "fullstack", ID: "fs-api-ui-errors", SignalDef: SignalDef{
		Pattern: domainPatternFromLexicon("fs-api-ui-errors", "build"),
		Weight:  2, Class: Medium,
	}},
	{Domain: "fullstack", ID: "fs-tenant-isolation", SignalDef: SignalDef{
		Pattern: domainPatternFromLexicon("fs-tenant-isolation", "plan"),
		Weight:  3, Class: Complex,
	}},
	{Domain: "devops", ID: "ops-hpa-autoscale", SignalDef: SignalDef{
		Pattern: domainPatternFromLexicon("ops-hpa-autoscale", "automate"),
		Weight:  2, Class: Medium,
	}},
	{Domain: "devops", ID: "ops-dns-discovery", SignalDef: SignalDef{
		Pattern: domainPatternFromLexicon("ops-dns-discovery", "inspect"),
		Weight:  2, Class: Medium,
	}},
	{Domain: "devops", ID: "ops-container-permissions", SignalDef: SignalDef{
		Pattern: domainPatternFromLexicon("ops-container-permissions", "protect"),
		Weight:  2, Class: Medium,
	}},
	{Domain: "devops", ID: "ops-config-reload", SignalDef: SignalDef{
		Pattern: domainPatternFromLexicon("ops-config-reload", "automate"),
		Weight:  2, Class: Medium,
	}},
	{Domain: "devops", ID: "ops-volume-expansion", SignalDef: SignalDef{
		Pattern: domainPatternFromLexicon("ops-volume-expansion", "plan"),
		Weight:  3, Class: Complex,
	}},
	{Domain: "devops", ID: "ops-service-mesh-mtls", SignalDef: SignalDef{
		Pattern: domainPatternFromLexicon("ops-service-mesh-mtls", "plan"),
		Weight:  3, Class: Complex,
	}},
	{Domain: "finops", ID: "finops-cost-tags", SignalDef: SignalDef{
		Pattern: domainPatternFromLexicon("finops-cost-tags", "build"),
		Weight:  2, Class: Medium,
	}},
	{Domain: "finops", ID: "finops-spot-usage", SignalDef: SignalDef{
		Pattern: domainPatternFromLexicon("finops-spot-usage", "optimize"),
		Weight:  2, Class: Medium,
	}},
	{Domain: "finops", ID: "finops-idle-resources", SignalDef: SignalDef{
		Pattern: domainPatternFromLexicon("finops-idle-resources", "inspect"),
		Weight:  2, Class: Medium,
	}},
	{Domain: "finops", ID: "finops-budget-anomalies", SignalDef: SignalDef{
		Pattern: domainPatternFromLexicon("finops-budget-anomalies", "build"),
		Weight:  2, Class: Medium,
	}},
	{Domain: "finops", ID: "finops-commitment-portfolio", SignalDef: SignalDef{
		Pattern: domainPatternFromLexicon("finops-commitment-portfolio", "plan"),
		Weight:  2, Class: Medium,
	}},
	{Domain: "finops", ID: "finops-cross-region-egress", SignalDef: SignalDef{
		Pattern: domainPatternFromLexicon("finops-cross-region-egress", "optimize"),
		Weight:  2, Class: Medium,
	}},
	{Domain: "devsecops", ID: "sec-csp-policy", SignalDef: SignalDef{
		Pattern: domainPatternFromLexicon("sec-csp-policy", "protect"),
		Weight:  2, Class: Medium,
	}},
	{Domain: "devsecops", ID: "sec-dependency-cve", SignalDef: SignalDef{
		Pattern: domainPatternFromLexicon("sec-dependency-cve", "inspect"),
		Weight:  2, Class: Medium,
	}},
	{Domain: "devsecops", ID: "sec-workload-identity", SignalDef: SignalDef{
		Pattern: domainPatternFromLexicon("sec-workload-identity", "protect"),
		Weight:  2, Class: Medium,
	}},
	{Domain: "devsecops", ID: "sec-sbom-license", SignalDef: SignalDef{
		Pattern: domainPatternFromLexicon("sec-sbom-license", "verify"),
		Weight:  2, Class: Medium,
	}},
	{Domain: "devsecops", ID: "sec-cross-cluster-rbac", SignalDef: SignalDef{
		Pattern: domainPatternFromLexicon("sec-cross-cluster-rbac", "plan"),
		Weight:  3, Class: Complex,
	}},
	{Domain: "devsecops", ID: "sec-key-rotation-policy", SignalDef: SignalDef{
		Pattern: domainPatternFromLexicon("sec-key-rotation-policy", "protect"),
		Weight:  2, Class: Medium,
	}},
	{Domain: "mobile", ID: "mob-push-delivery", SignalDef: SignalDef{
		Pattern: domainPatternFromLexicon("mob-push-delivery", "build"),
		Weight:  2, Class: Medium,
	}},
	{Domain: "mobile", ID: "mob-cold-start", SignalDef: SignalDef{
		Pattern: domainPatternFromLexicon("mob-cold-start", "optimize"),
		Weight:  2, Class: Medium,
	}},
	{Domain: "mobile", ID: "mob-background-location", SignalDef: SignalDef{
		Pattern: domainPatternFromLexicon("mob-background-location", "build"),
		Weight:  2, Class: Medium,
	}},
	{Domain: "mobile", ID: "mob-testflight-release", SignalDef: SignalDef{
		Pattern: domainPatternFromLexicon("mob-testflight-release", "automate"),
		Weight:  2, Class: Medium,
	}},
	{Domain: "mobile", ID: "mob-offline-db-migration", SignalDef: SignalDef{
		Pattern: domainPatternFromLexicon("mob-offline-db-migration", "plan"),
		Weight:  3, Class: Complex,
	}},
	{Domain: "mobile", ID: "mob-battery-background", SignalDef: SignalDef{
		Pattern: domainPatternFromLexicon("mob-battery-background", "optimize"),
		Weight:  2, Class: Medium,
	}},
	{Domain: "cicd", ID: "ci-actions-permissions", SignalDef: SignalDef{
		Pattern: domainPatternFromLexicon("ci-actions-permissions", "protect"),
		Weight:  2, Class: Medium,
	}},
	{Domain: "cicd", ID: "ci-artifact-provenance", SignalDef: SignalDef{
		Pattern: domainPatternFromLexicon("ci-artifact-provenance", "verify"),
		Weight:  2, Class: Medium,
	}},
	{Domain: "cicd", ID: "ci-monorepo-cache", SignalDef: SignalDef{
		Pattern: domainPatternFromLexicon("ci-monorepo-cache", "optimize"),
		Weight:  2, Class: Medium,
	}},
	{Domain: "cicd", ID: "ci-semver-release", SignalDef: SignalDef{
		Pattern: domainPatternFromLexicon("ci-semver-release", "automate"),
		Weight:  2, Class: Medium,
	}},
	{Domain: "cicd", ID: "ci-affected-matrix", SignalDef: SignalDef{
		Pattern: domainPatternFromLexicon("ci-affected-matrix", "build"),
		Weight:  2, Class: Medium,
	}},
	{Domain: "cicd", ID: "ci-secret-exposure", SignalDef: SignalDef{
		Pattern: domainPatternFromLexicon("ci-secret-exposure", "inspect"),
		Weight:  2, Class: Medium,
	}},
	{Domain: "testing", ID: "qa-consumer-contract", SignalDef: SignalDef{
		Pattern: domainPatternFromLexicon("qa-consumer-contract", "verify"),
		Weight:  2, Class: Medium,
	}},
	{Domain: "testing", ID: "qa-property-based", SignalDef: SignalDef{
		Pattern: domainPatternFromLexicon("qa-property-based", "build"),
		Weight:  2, Class: Medium,
	}},
	{Domain: "testing", ID: "qa-load-p99", SignalDef: SignalDef{
		Pattern: domainPatternFromLexicon("qa-load-p99", "verify"),
		Weight:  2, Class: Medium,
	}},
	{Domain: "testing", ID: "qa-e2e-flakiness", SignalDef: SignalDef{
		Pattern: domainPatternFromLexicon("qa-e2e-flakiness", "inspect"),
		Weight:  2, Class: Medium,
	}},
	{Domain: "data", ID: "data-cdc-multi-db", SignalDef: SignalDef{
		Pattern: domainPatternFromLexicon("data-cdc-multi-db", "plan"),
		Weight:  3, Class: Complex,
	}},
	{Domain: "data", ID: "data-online-backfill", SignalDef: SignalDef{
		Pattern: domainPatternFromLexicon("data-online-backfill", "plan"),
		Weight:  3, Class: Complex,
	}},
	{Domain: "data", ID: "data-quality-constraints", SignalDef: SignalDef{
		Pattern: domainPatternFromLexicon("data-quality-constraints", "verify"),
		Weight:  2, Class: Medium,
	}},
	{Domain: "data", ID: "data-stream-consumer-lag", SignalDef: SignalDef{
		Pattern: domainPatternFromLexicon("data-stream-consumer-lag", "inspect"),
		Weight:  2, Class: Medium,
	}},
	{Domain: "ai", ID: "ai-rag-retrieval", SignalDef: SignalDef{
		Pattern: domainPatternFromLexicon("ai-rag-retrieval", "optimize"),
		Weight:  2, Class: Medium,
	}},
	{Domain: "ai", ID: "ai-agent-tool-errors", SignalDef: SignalDef{
		Pattern: domainPatternFromLexicon("ai-agent-tool-errors", "inspect"),
		Weight:  2, Class: Medium,
	}},
	{Domain: "ai", ID: "ai-eval-regression-set", SignalDef: SignalDef{
		Pattern: domainPatternFromLexicon("ai-eval-regression-set", "build"),
		Weight:  2, Class: Medium,
	}},
	{Domain: "ai", ID: "ai-multi-agent-hitl", SignalDef: SignalDef{
		Pattern: domainPatternFromLexicon("ai-multi-agent-hitl", "plan"),
		Weight:  3, Class: Complex,
	}},
	{Domain: "observability", ID: "obs-trace-correlation", SignalDef: SignalDef{
		Pattern: domainPatternFromLexicon("obs-trace-correlation", "build"),
		Weight:  2, Class: Medium,
	}},
	{Domain: "observability", ID: "obs-cross-service-slo", SignalDef: SignalDef{
		Pattern: domainPatternFromLexicon("obs-cross-service-slo", "plan"),
		Weight:  3, Class: Complex,
	}},
	{Domain: "observability", ID: "obs-metric-cardinality", SignalDef: SignalDef{
		Pattern: domainPatternFromLexicon("obs-metric-cardinality", "inspect"),
		Weight:  2, Class: Medium,
	}},
	{Domain: "observability", ID: "obs-alert-fatigue", SignalDef: SignalDef{
		Pattern: domainPatternFromLexicon("obs-alert-fatigue", "optimize"),
		Weight:  2, Class: Medium,
	}},
}
