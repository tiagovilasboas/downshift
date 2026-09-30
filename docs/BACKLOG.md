# Autonomous execution backlog

`harness-downshift` by Tiago de Carvalho Vilas Boas  
https://github.com/tiagovilasboas/harness-downshift

This is the handoff contract for autonomous subagent work. It records the
current state, order, boundaries and evidence required to advance each item.
It is not proof that a capability has been accepted by an executor.

## Execution rules

| Rule | Contract |
| --- | --- |
| Order | P0 → P1 telemetry consumers → P1 orchestration acceptance → P2 experiments/governance. A blocked item does not authorize bypassing its dependency. |
| Ownership | AI/FinOps owns policy, cost, evaluation and cross-stream integration; Full-stack owns bounded runtime integration; AppSec owns data/tool boundary review. |
| Autonomy | Local code, docs and tests only. Preserve unrelated dirty files. Do not push, merge, publish, alter corporate KB rules, add credentials, call external model services or enable a provider without explicit user authorization. |
| Stop/escalate | Stop and report when a native ACK contract is unavailable, an API/account/data-transfer is needed, a policy would weaken a hard gate, test evidence conflicts, or a change touches user-owned dirty files. |
| Commit policy | No commit or push while this backlog is active. When separately authorized and a unit is accepted, stage only its named files, run `git status --short`, use an English scoped commit message, then report its SHA before any push. |
| Evidence | Record commands and exit status, affected paths, privacy classification, correlation/ACK semantics, and limitations. `rewrite_emitted` never proves a vendor applied a model. |

## State and dependency map

| ID | Priority | State | Depends on | Owner | Stop condition |
| --- | --- | --- | --- | --- | --- |
| DS-01 | P0 | Accepted as local `rewrite_emitted` | — | Full-stack + AppSec | — |
| DS-02 | P1 | Accepted; shadow-only telemetry integrated with `Apply:false` | DS-01 event schema | AI/FinOps + Full-stack | any proposal to bypass hard gates |
| DS-03 | P1 | ✅ Ported to Go (internal/orchestration/); Python graph is reference only | DS-01 | Full-stack + AppSec | — |
| HC-01 | P1 | Accepted as local provider-neutral foundation | DS-01 accepted | AI/FinOps + Full-stack + AppSec | provider/retrieval integration requested |
| KB-01 | P2 | Accepted local structured summary/telemetry instrumentation | HC-01 accepted | AI/FinOps + Full-stack | retrieval boundary cannot be evidenced prompt-free |
| KB-02 | P2 | Accepted local boundary fixtures/guardrails | KB-01 contract and AppSec review | AppSec + AI/FinOps | personal/corporate boundary would be weakened |
| DS-04 | P3 | POC local em execução: lifecycle observation, sem ACK | contrato de privacidade + lifecycle Codex | Full-stack + AppSec | correlação ambígua ou ausência de contrato suportado |

## DS-01 — P0 local runtime evidence

- **Files:** `internal/telemetry/`, `internal/adapters/{codex,cursor,claudecode}/`, adapter E2E tests and `docs/ENGINEERING-LOOP.md`.
- **Scope:** local evidence records the emitted rewrite mapped to `correlation_id`, requested/final model and reasoning effort.
- **Exclusions:** no inference proxy, no fabricated acknowledgement from hook output/stderr, no prompt or secret persistence.
- **Commands:** `go test -race ./...`; `go vet ./...`; `git diff --check`; a per-adapter E2E test with matching acknowledgement ID.
- **Acceptance:** accepted as `rewrite_emitted`; it is not proof of executor-applied model.
- **Installed evidence:** local ARM64 binary was atomically refreshed from worktree
  `144a71315e634a57c9ac35b063e4fcbaef8c936a`; installed/build SHA-256 is
  `2bff6eed3b1f650bae98d906b9d7b8bb165b779735a34f3a671fa021cacb4dd3`.
  A dated backup of the prior binary exists beside the installation. Hook matcher
  and trust were read-only verified; isolated `try` and Codex-hook smoke tests
  produced prompt-free `rewrite_emitted` telemetry.
- **Fowler:** guia inferencial / architecture fitness: outcome semantics; sensor computacional / behaviour: correlated E2E test; continuous prompt-free telemetry: sensor computacional / behaviour.

## DS-02 — P1 Decision Intelligence shadow telemetry

- **Current state:** accepted and integrated as prompt-free shadow telemetry. `internal/decisionintelligence/` accepts structured risk, sensitivity, budget, confidence and **reviewed** outcome signals; it can preserve or elevate tier only; `Apply:false` makes it advisory. `JevAdapter` is disabled by default with no SDK, I/O, account or API key.
- **Files:** `internal/decisionintelligence/`, `internal/telemetry/`, tests, `docs/DECISION-INTELLIGENCE.md` and this backlog.
- **Scope:** emit allowlisted recommendation metadata in shadow mode beside the deterministic decision, then compare with reviewed feedback.
- **Exclusions:** no automatic rewrite, no LLM/Jev critical path, no provider selection, no permission/budget/safety enforcement in this package.
- **Commands:** `go test -race ./...`; `go vet ./...`; `git diff --check`; JSONL schema/E2E test proving no prompt fields and no effect on final policy.
- **Acceptance:** recommendation/correlation evidence exists; a hard-gate test proves advisory data cannot lower a final tier or authorize a route; sensitivity never chooses a provider.
- **Fowler:** guia computacional / behaviour: typed monotonic contract; sensor computacional / behaviour: schema + E2E tests; sensor inferencial / architecture fitness: periodic decision-quality review.

## DS-03 — P1 LangGraph acceptance (✅ ported to Go)

- **Files:** `internal/orchestration/planner.go`, `docs/LANGGRAPH-ORCHESTRATION.md`. Python reference kept at `orchestration/`.
- **Scope:** fan-out delegation planning (dedup, ordering, max_delegates, capability limits) ported to Go. 14/14 tests green. Zero Python, zero latency overhead.
- **Exclusions:** Python LangGraph kept as reference/training artifact — not the execution path. Provider, model, LLM calls remain outside this package.
- **Acceptance:** ✅ Go planner validated 2026-09-30. Python graph is reference only.

## HC-01 — P1 LangChain in Harness Central

- **Files:** `/Users/tiago.boas/Github/harness-core/context_runtime/`, its docs and CI only. Do not copy corporate rules from `voomp-kb`.
- **Scope:** local, opt-in provider-neutral context-contract foundation. It does not retrieve or transmit sources itself.
- **Exclusions:** no universal memory, no silent prompt/context export, no LangChain dependency in Downshift's hook critical path.
- **Commands:** locked module tests, `git diff --check`, payload/provenance limit tests and pre-chain redaction test.
- **Acceptance:** accepted as a local foundation: limits, provenance budget and redaction pre-chain are tested and AppSec-approved. It still does not retrieve or transmit content.
- **Fowler:** guia inferencial / architecture fitness: source-selection contract; sensor computacional / behaviour: provenance test; sensor inferencial / architecture fitness: review of provider/tool boundary.

## KB-01 — P2 RAG token compression

- **Files:** Harness Central contract/docs and `rag-kb` architecture page only after hub-first retrieval confirms the appropriate personal scope.
- **Scope:** incremental factual summaries plus selective retrieval, with an explicit retention and freshness policy.
- **Exclusions:** no raw chat history or secret storage; no treating Codex runtime memory as shared RAG truth.
- **Commands:** source allowlist test, stale-summary test and `git diff --check`.
- **Acceptance:** every injected fact has a source/version; retrieval can demonstrate lower context payload without omitting required constraints.
- **Fowler:** guia inferencial / architecture fitness: retrieval contract; sensor computacional / behaviour: source/freshness tests; continuous sensor computacional / maintainability: context-size and cache-hit metrics.

## KB-02 — P2 cross-KB guardrails

- **Files:** Harness Central steering/contracts and `rag-kb` boundary docs; `voomp-kb` remains source-controlled corporate knowledge and is not altered here.
- **Scope:** enforce `rag-kb` personal/operational versus `voomp-kb` corporate business-rule selection with provenance, allowlists and redaction rules.
- **Exclusions:** no bidirectional sync, no duplication of corporate rules, no Fowler treatise in `voomp-kb`.
- **Commands:** boundary fixture tests, provenance/redaction test, `git diff --check`, and AppSec review of the diff.
- **Acceptance:** forbidden cross-scope retrieval is rejected and evidenced; allowed Voomp rule references preserve `arquivo@sha`; personal pages never become a corporate rule source.
- **Fowler:** guia computacional / behaviour: source allowlists; guia inferencial / architecture fitness: boundary steering; sensor computacional / behaviour: fixture tests; sensor inferencial / behaviour: AppSec review.

## DS-04 — P3 native executor ACK investigation

- **Scope:** POC local, disabled-by-default, com fixtures de lifecycle Codex.
  Ele pode reportar somente `unmatched`, `ambiguous` ou `observed`; não pode
  declarar `executor_acknowledged`.
- **Exclusions:** no inferred ACK, transcript scraping as a stable API, runtime
  environment probing without a documented contract, or collection enabled by default.
- **Dependencies:** explicit privacy policy, documented vendor lifecycle/trace
  contract, deterministic correlation under concurrent spawns and AppSec review.
- **Acceptance:** a harness-specific POC produces a documented correlation and
  can distinguish `rewrite_emitted` from `executor_acknowledged`; otherwise it
  records the external limitation and remains backlog-only.
- **Fowler:** guia inferencial / architecture fitness: per-vendor evidence
  contract; sensor computacional / behaviour: concurrent-spawn correlation test;
  sensor inferencial / behaviour: privacy/AppSec review.
