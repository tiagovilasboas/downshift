# Domain: Subagent Model Delegation

Fail-open, deterministic application of the orchestrator's model choice for delegated subagent tasks in coding harnesses. Owner decision O8 (2026-10-28): the orchestrator LLM chooses agent, model and effort; Downshift applies the choice, enforces the user's `policy.json` and measures. Downshift never picks a model on its own.

Downshift optimizes agent costs where coding harnesses spend the most: delegated execution. When an orchestrator or main session spawns a subagent (`Task`, `Agent`, `spawn_agent`, or `subagent`), Downshift intercepts the hook and applies the orchestrator's choice (pinned model kept, then declared profile, first matching declared rule, the orchestrator's tier hint down only for now, else the requested model is kept), bounded by the user's declared policy. Its own code makes no LLM call. Complexity and risk are still classified, as telemetry and as the key of user-declared floors, not as a decision.

---

## 1. Core Principles & Philosophy

1. **Deterministic & Fast**: Sub-millisecond execution on the hot path. Downshift's code makes no LLM call, no network call and runs no heavy runtime; the orchestrator's choice is applied deterministically.
2. **Fail-Open by Design**: Any unexpected error, unhandled exception, syntax issue, or ambiguity results in an immediate pass-through (the harness executes the model it originally requested).
3. **Conservative Under Doubt**: Doubt never spends money. Unconfident upshifts and unconfident frontier model choices are held.
4. **Session-First Candidate Set**: The session defines what models exist. Exact session metadata wins, and the internal catalog serves only as a fallback.
5. **Prompt Privacy**: Task instructions and prompt texts are evaluated in-memory only. Prompts are never written to disk, telemetry logs, or remote servers.

---

## 2. The Routing Loop

```
[Main Session / Orchestrator]
       │
       ▼ (fires delegation hook: PreToolUse / agent.spawn)
[1. Intercept] ──(reads task text in-memory only)
       │
       ▼
[2. Classify] ──(telemetry only since O8: deterministic signals + word-hash boost)
       │         └── maps to TRIVIAL, SIMPLE, MEDIUM, or COMPLEX
       │         └── evaluates Language Guard (EN & PT-BR) & Risk Categories
       ▼
[3. Guardrails] ──(R1..R9 invariants: doubt never spends money)
       │          └── enforces project tier floor (.downshift/policy.json)
       │          └── checks quota snapshot & user-declared floors (no built-in risk floor since O8)
       ▼
[4. Apply] ──(applies the orchestrator's choice and declared policy; else keeps the request)
       │
       ▼
[5. Emit / Pass-through]
       ├── If confident & safe: emits updatedInput.model / reasoning_effort
       └── If uncertain or error: passes through unchanged (fail-open)
```

### 2.1 Classification Signals
- **Deterministic Regex Signals**: Evaluates weighted pattern matches across English and Portuguese vocabularies (`internal/classify/signals.go`).
- **In-Process Semantic Boost**: An FNV word-hash embedder checks prompt similarity against known centroids. It can raise an uncertain tier but can never lower a tier, never boost a prompt with no signals, and never elevate to COMPLEX alone (`DOWNSHIFT_MINILM=0` disables).
- **Language Guard**: Heuristic detector for unsupported languages and mixed-language inputs. Drops confidence when outside covered vocabularies, ensuring uncertain spawns are held.
- **Risk categories**: Tasks touching secrets, payment processing, authentication, data loss, or destructive operations are recorded as telemetry. The built-in risk floor and veto were removed on 2026-10-28 (O8): the protection is the orchestrator's judgment, guided by the `downshift-orchestrator` playbook, and floors the user declares in `policy.json`.

---

## 3. BDI Agentic View (Beliefs → Desires → Intentions)

BDI serves as an architectural lens for reasoning about Downshift's deterministic state transitions:

| BDI Phase | Downshift Component | Implementation Packages |
|---|---|---|
| **Perception** | Adapter extracts PreToolUse payload (task prompt, model, session state) | `internal/adapters/*`, `runHookAdapter` |
| **Beliefs** | Evaluated complexity, confidence score, risk tags, language validity, session candidate metadata, quota | `internal/classify`, `internal/session`, `internal/core/session_resolver.go`, `internal/catalog` |
| **Desires** | Match capability to task; prevent overspend on uncertainty; respect project policy floors | `EscalationIntent`, `selfcheck.go` |
| **Deliberation** | Deterministic policy evaluation (`Route` + `Check`) | `internal/core/policy.go` |
| **Intention** | Synthesizes Decision + SafeVerdict + SessionPlan into concrete RewritePlan | `Decision`, `RewritePlan` |
| **Action** | Encodes harness-specific mutation (`updatedInput.model`, `reasoning_effort`) | Adapter encoders |
| **Feedback** | Records observation: rewrite honored, token usage, child ending status | `internal/hookport`, `internal/telemetry` |

---

## 4. Guardrails & Declared Floors

- **R1–R9 Invariant Rules**: Enforced by automated invariant suites (`cost_safety_invariants_test.go`). Unconfident downshifts, unconfident upshifts, and unconfident frontier selections are held at baseline.
- **Project Tier Floor**: Repositories can define `.downshift/policy.json` to enforce minimum model tiers per task class or risk profile. A project floor can only raise a tier, never lower one.
- **Quota & Credit Gate**: Prevents downshifting or upshifting to models where active credit or quota is exhausted, relying on recent provider usage snapshots (`docs/guides/quota.md`).

---

## 5. Model Catalog & Resolution

Model definitions reside in embedded metadata (`internal/catalog/catalog.json`) covering Anthropic, OpenAI, Gemini, Grok, and Cursor families.

- **Prices and Freshness**: The catalog carries `prices_as_of`; the weekly `sync-prices` workflow opens a PR when a list price changes, and `downshift doctor` warns after 30 days. The hook never fetches a price ([policy](../reference/prices.md)).
- **Exact Match → Alias → Family Prefix**: Resolves known models, aliases, and new point releases in the same family.
- **Explicit Only**: Specialized high-cost or niche models (e.g., `gpt-6-astra`, `claude-fable-5-1-thinking-high`) are flagged `explicit_only` and are never auto-selected by Downshift.
- **Session Model Inference**: When the harness does not provide an explicit candidate list, Downshift infers the candidate ladder below the currently active model (ensuring it never upshifts above the session's chosen model).

---

## 6. Harness Support Status

| Harness | Status | Tool Intercepted | Mutation Mechanism | Evidence / Validation |
|---|---|---|---|---|
| **Claude Code** | Completed | `Task` / `Agent` | `PreToolUse` → `updatedInput.model` | Observed honored (2026-10-06); transcript token usage; controlled A/B evidence |
| **Codex** | Functional | `spawn_agent` | `PreToolUse` → model + `reasoning_effort` | Inferred, conformance test verified |
| **Cursor** | Functional | `Task` | `preToolUse` → `updated_input.model` | Conformance verified; unconfirmed on Free/legacy Pro |
| **OpenCode** | Beta | `subagent` | Hook adapter / plugin wire | Adapter and conformance tests in repository |
| **Antigravity** | Experimental | `Task` | Policy mode / native hook | Single-build observed execution |
| **KiroCrew** | Experimental | Native CLI hook | Allow/block retry | Single-build observed execution |

---

## 7. Deep References

- [Architecture & Execution Flow](../design/architecture.md): Deep-dive into internal interfaces, package boundaries, and execution sequences.
- [Complexity Weights & Signals](../reference/complexity-weights.md): Full listing of regex weights, semantic centroids, and language rules.
- [Session Models & Discovery](../reference/session-models.md): Catalog formats, session inference rules, and Claude Code transcript parsers.
- [Harness Compatibility Matrix](../reference/harness-matrix.md): Exhaustive breakdown of plans, hooks, and observed status across harnesses.
- [Subagent Routing A/B Evidence](../evidence/subagent-routing-ab-2026-10-15.md): Empirical test results from controlled routing trials.
