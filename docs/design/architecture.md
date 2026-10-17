# Architecture — Downshift

High-level view for adopters and contributors. Implementation details live in
code and [CONTRIBUTING.md](../../CONTRIBUTING.md).

Downshift by Tiago de Carvalho Vilas Boas · https://github.com/tiagovilasboas/downshift

---

## What Downshift is

A **single Go binary** (`downshift`) that runs as a **harness hook** (JSON on
stdin, JSON on stdout) and as an offline CLI over its own event log. On the hook
path it classifies the task a main agent delegates, applies policy, catalog and
session rules, and returns an **updated tool input** with `model` (and, where
the harness takes it, `reasoning_effort`) rewritten. Around that one decision
it keeps the evidence needed to say whether the rewrite was honored, what the
child cost, and whether routing beat a measured baseline.

Read it as an **optimization layer for agent execution**, not only a router:
one deterministic decision per delegation, guardrails that hold under doubt,
observation of what the harness then did, and offline evaluation that feeds
the next policy change. The hook itself stays small: **no network, no LLM, no
prompt storage** on the hot path, and every failure fails open.

---

## End-to-end flow

Each box names its owner: `[D]` Downshift, `[H]` harness-native, `[X]` an
external tool. Dotted boxes are opt-in or shadow and never change the route.

```text
 TASK ──▶ main agent plans and executes [H]
             │ delegates: Task/Agent, spawn_agent, invoke_subagent, spawn_run [H]
             ▼
 PreToolUse hook [H] ─ stdin JSON ─▶ runHookAdapter + adapter [D]
             │  bounded read (1 MiB), parse, hook deadline, ProjectDir seam, panic → allow
             ▼
 context and reference model [D]
   current model: tool input → payload → override env → transcript tail →
   settings fallback (flagged as a guess)        [claude-code adapter]
   session list: hook lists → session-models.json → discovery → inferred from
   the current model (Claude Code only) · quota marks from the hook, quota.json,
   native exports (never fetched during a hook)
             ▼
 classify + risk [D]  Classify (signals) → semantic boost → risk floor /
   abstain → graphify escalation → NB second opinion ┆ E5 shadow (observe) ┆
             ▼
 Route + Check [D]  tier → effort → catalog model → verdict vs current →
   R1 R2 R4 R5 R6 R7 R8 R9 (any hit ⇒ OK, keep current) → project floor
   (.downshift/policy.json, re-checked) ┆ adapt memory, score path (shadow/opt-in) ┆
             ▼
 PlanSession [D]  foreign id → hold · explicit_only → preserve · unknown
   session → keep · pick the session model at the tier (exhausted quota never
   selected) · CanWriteSessionID · StrictModelName → native name
             ▼
 fate [D]  control group (10%) / per-session pause / DOWNSHIFT_NO_ROUTE ⇒ allow
   unchanged, logged as baseline; else emit updatedInput (model, effort) or,
   for kirocrew, exit 2 + stderr policy
             │  stderr: correlation id, decision summary, feedback id
             ▼
 harness spawns the subagent [H]  (may honor, drop or reject the rewrite:
   docs/reference/harness-matrix.md)
             ▼
 child end [D ← H]  PostToolUse: resolvedModel (honored / refuted), no tokens for
   async children · SubagentStop: child transcript → tokens, catalog price ·
   `downshift end <harness>` is the same recorder; Cursor/Codex ends unverified
             ▼
 event log [D]  ~/.downshift/events.jsonl, prompt-free · loop-events.jsonl
   (features + tier, for feedback) · sensor: tool output bytes (observe only)
             ▼
 read-outs [D]  stats (list-price estimate) · report (routed vs control, cost per
   completed task, rework, bootstrap CI) · session / watch / serve (local
   dashboard over the log) · doctor and skills (diagnosis)
             ▼
 tuning [D, offline]  benchmark (classifier KPIs on labelled fixtures) ·
   eval-outcome · feedback → reviewed required_tier labels → train ·
   shadow-report · classifier tests
```

![Architecture](../img/architecture.svg)

BDI agentic loop (beliefs / desires / intentions on the hook path):

![BDI agentic router](../img/bdi-agentic-light.svg)

### Step by step, with what the code does

1. **Task, planning, delegation** `[H]`. The main agent decides on its own
   when to delegate. Downshift sees nothing until the harness fires the hook
   on the spawn tool (`Agent|Task`, `spawn_agent`, `invoke_subagent`,
   `spawn_run`; `cmd/downshift/setup_targets.go`).
2. **Hook runner and adapter** `[D]`. `runHookAdapter` (`cmd/downshift/main.go`)
   is shared by every harness through `harnessRegistry` (`harnesses.go`). It
   bounds stdin, parses the payload, sets the hook deadline and `ProjectDir`,
   recovers a panic into an allow, and only then calls the adapter's `Handle`.
   Adapters (`internal/adapters/<harness>`) parse fields, declare
   `HarnessCapabilities`, call core and encode the wire response. No policy.
3. **Reference model and session** `[D]`. The Claude Code adapter resolves the
   model the child would have run on: the call's `model`, the payload model,
   `CLAUDE_CODE_SUBAGENT_MODEL` for inheriting agent types, the last assistant
   message in the parent transcript tail (bounded 400 ms wait), and finally
   the configured model (`ANTHROPIC_MODEL`, `settings*.json`), which is marked
   as a guess and may only downshift to the small tier. The session list comes
   from hook lists, `session-models.json`, discovery, or (Claude Code only)
   inference from the current model, never above it. Quota marks come from the
   hook payload, the `quota.json` cache or an opt-in native export file; a hook
   never calls a provider.
4. **Classification and risk** `[D]`. `classifyTask` (`internal/core/policy.go`)
   runs the signal scorer, the monotonic semantic boost, the risk floor and
   risk abstention (`risk.go`), graphify escalation of unconfident tasks, and
   the naive-Bayes second opinion. The E5 shadow records an observation only.
5. **Route, guardrails, floor** `[D]`. `Route` resolves tier, effort and the
   catalog model, compares with the current model, then `Check`
   (`selfcheck.go`) applies R1, R2, R4, R5, R6, R7, R8, R9 (R3 retired): any
   violation turns the safe verdict into OK, which keeps the current model.
   Only a deterministic lift (risk, graph, opted-in NB upshift, operator floor)
   may move up. The project floor (`internal/policyfile`, `TierFloorHook`) runs
   after `Check` and re-checks. Adapt memory records a would-be tier and does
   not change it; the capability score path runs only with
   `DOWNSHIFT_SCORE_ROUTING=1` and behind the same safe-verdict gate.
6. **Session plan and fate** `[D]`. `PlanSession` holds foreign ids, preserves
   `explicit_only` models, keeps the model when the session is unknown, and
   selects the session model at the needed tier, skipping exhausted quota. The
   runner then applies the control group (`DOWNSHIFT_CONTROL_SHARE`, default
   0.10), the per-session pause and `DOWNSHIFT_NO_ROUTE`: those spawns pass
   unchanged and are logged as `baseline`. Otherwise the wire emits
   `updatedInput` with only `model` and `reasoning_effort` changed; kirocrew
   has no rewrite channel and gets a block-with-reason instead.
7. **Subagent execution** `[H]`. The harness decides whether the rewrite takes
   effect. Honored on Claude Code, Antigravity and (inferred) Codex; dropped on
   Cursor Free and legacy Pro; config-only on Grok. Downshift does not run the
   child, cannot see its context, and never edits what it returns.
8. **Child end and usage** `[D]` from `[H]` payloads. PostToolUse on Claude Code
   is the launch of an async child: it carries `resolvedModel` (honored or
   refuted) and no tokens. SubagentStop supplies the child transcript, from
   which the shared recorder (`internal/childend`) prices tokens with catalog
   list prices. A nil port stays unobserved; absence is never inferred.
   Latency is not measured per step: the only `latency_ms` in the log is the
   E5 shadow's own.
9. **Telemetry and read-outs** `[D]`. `events.jsonl` holds routing metadata
   only (no prompts, tasks or paths). `stats` estimates at list price;
   `report` compares routed spawns with the control group, counting "completed"
   only for harnesses that declare an end event and have end records;
   `session`, `watch` and `serve` read the same log. `serve` is a local HTTP
   dashboard, not a proxy.
10. **Benchmarks and tuning** `[D]`, offline. `benchmark` scores the classifier
    on labelled fixtures (tier accuracy, unsafe downgrade, over-routing);
    `eval-outcome` runs the outcome suite (minimal fixtures in OSS, full suite
    in downshift-labs); `feedback` attaches success/retry/failed to a routing
    id, and only reviewed `required_tier` labels enter `train`; `shadow-report`
    compares candidate classifiers that never touched a route.

---

## Boundaries

| Concern | What Downshift does | Owner | Where |
|---|---|---|---|
| Harness session policy | `setup` offers `model: opusplan`, `permissions.defaultMode: plan`, `enabledPlugins["cc-plugin-you-should-know@builtin"]: true` for Claude Code, each written only when the key is unset; `--no-session-policy` skips it. The main agent's model is otherwise untouched. | D writes once, H enforces | `cmd/downshift/setup_targets.go` |
| Subagent routing | The flow above: one decision per spawn, rewrite or allow, fail open. | D decides, H applies | `internal/core`, adapters, `runHookAdapter` |
| Context optimization | Native provider is **observe-only**: PostToolUse records tool output bytes and the reduction `safe` mode would reach; the hook never replaces the tool result the model sees. `downshift context compress` compacts piped output in `safe` mode as an explicit CLI step. Headroom is a companion: detected on PATH, recorded with `context enable headroom`, never installed, launched or wrapped. | D observes; X compresses | `internal/contextopt`, `internal/compressor`, `internal/sensor`, `providers/headroom` |
| Observability | Prompt-free `events.jsonl`; honor, resolved, child_ended and usage records; `stats`, `report`, `session`, `watch`, `serve`, `probe` (maintainer). | D | `internal/telemetry`, `internal/childend`, `internal/server` |
| Diagnosis and recommendations | `doctor` (binary, state dir, catalog, hooks per harness, model override, quota, policy, context optimization, companion, semantic boost) with a `fix` per failed check; skills `downshift-setup`, `downshift-audit`, `cost-reducer` write nothing without a yes. | D | `cmd/downshift/doctor*.go`, `skills/` |
| Benchmark and economic evaluation | Classifier KPIs, outcome evals, control-group report with bootstrap CI, feedback and offline training. Public `benchmark/` holds the published report; datasets stay in downshift-labs. | D | `internal/benchmark`, `internal/outcome`, `internal/routingv2/training` |

### Shadow and opt-in paths

| Path | Status | Switch |
|---|---|---|
| E5 shadow / fuse | observation on `Route()`; never changes tier | `DOWNSHIFT_E5_SHADOW`, `DOWNSHIFT_E5_FUSE` |
| adapt memory (`internal/adapt`) | records a would-be tier; a present file does not change `Tier` | file under the state dir |
| capability score routing | live only when enabled, behind the safe-verdict gate | `DOWNSHIFT_SCORE_ROUTING=1` |
| NB upshift | opinion always logged; applied only when enabled | `DOWNSHIFT_NB_UPSHIFT=1` |
| candidate classifier (`routingv2/shadow`) | observe; `shadow-report` compares | `DOWNSHIFT_SHADOW_WEIGHTS` |
| `decisionintelligence` | advisory record on every event, `Apply: false` | none |
| lifecycle observer, LangGraph `orchestration/` | disabled by default; pick no models | config |

---

## What Downshift is not

- **Not an HTTP proxy.** Nothing sits between the harness and the model API.
  `downshift serve` is a local dashboard that reads the event log.
- **Not an LLM-in-the-loop router.** Classification is a scored signal table,
  a hash embedder and a naive-Bayes opinion, all local and deterministic.
- **Does not edit tool results in flight.** The PostToolUse hook returns `{}`
  (plus an optional status line); context compression is observe-only unless
  the user pipes output through `downshift context compress` themselves.
- **Does not store prompts.** Task text stays in memory for feature extraction;
  the event log, feedback store, probe and telemetry carry metadata only.
- **Does not run the subagent or guarantee the rewrite.** The harness executes
  and may ignore `updatedInput`; the matrix records what each one honors.

---

## Module boundaries

| Package | Responsibility |
|---------|----------------|
| `internal/core` | Classifier, risk, policy, escalation, self-check, session guards, `HarnessCapabilities` — **no** catalog, telemetry, adapt, policyfile or adapter imports |
| `internal/catalog` | Embedded `catalog.json`, user override, `Resolver` |
| `internal/adapters/*` | Harness-specific I/O only; call `core.Route` / `PlanSession` |
| `internal/policyfile`, `internal/adapt`, `internal/routeadapt` | Project tier floor (`TierFloorHook`) and shadow tier memory (`MemoryAdjustHook`), plugged into core at `init` |
| `internal/quota`, `internal/hookctx`, `internal/hookutil` | Offline budget evidence; per-hook deadline and project dir; shared field helpers |
| `internal/hookport`, `internal/childend` | Honor, usage and child-end contract; one shared recorder for every harness |
| `internal/telemetry` | Append-only local JSONL; prompt-free events; stats, report, control group, session tree |
| `internal/contextopt`, `internal/compressor`, `internal/sensor` | Context optimization registry, native compressor (observe/safe), tool-output sensor |
| `internal/semantic`, `internal/nbtier`, `internal/graphify` | Semantic boost, naive-Bayes opinion, critical-symbol escalation |
| `internal/routingv2/*`, `internal/decisionintelligence`, `internal/outcome`, `internal/benchmark` | Shadow classifiers and training, advisory recommendation, outcome evals, classifier KPIs |
| `internal/server` | Local dashboard over the event log (`serve`, `dsmon-server`) |
| `cmd/downshift` | CLI, `harnessRegistry`, `runHookAdapter`, `runEnd`, setup, doctor |

**Invariants:** adapters embed no routing rules; adding a harness is one
`harnessRegistry` entry plus its adapter package, with a port entry only when
that harness's own hook sends honor or usage fields; a nil port is unobserved,
never inferred. Dependency check: `go list -deps ./internal/core`.

---

## Configuration on disk

State directory `~/.downshift/` (legacy `~/.harness-downshift/`, see
[rename.md](../brand/rename.md); override with `DOWNSHIFT_STATE_DIR`):
`catalog.json` (user override), `events.jsonl`, `loop-events.jsonl`,
`session-models.json`, `quota.json`, `context-optimization.json`,
`probe.jsonl`, optional weights and scores cache. Per project:
`.downshift/policy.json` (tier floor). Overrides: `DOWNSHIFT_EVENT_LOG`,
`DOWNSHIFT_SESSION_MODELS`, `DOWNSHIFT_QUIET`, flags in README and
[minilm-semantic.md](minilm-semantic.md).

---

## Packaging layers (what an agent harness loads)

Besides the Go layers above, the repository is laid out as the five things a
coding harness can load. Each is a thin surface over the same binary; none holds
policy. The plugin is Claude Code only; every other harness reaches the same
layers through `downshift setup` and the docs.

| Layer | Path | Holds | Harness-neutral? |
|---|---|---|---|
| 1 Memory | `AGENTS.md` (`CLAUDE.md` points to it) | always-loaded rules, at most 200 lines | yes |
| 2 Skills | `skills/downshift-setup`, `skills/downshift-audit`, `skills/cost-reducer` | procedures with judgment; detail in `references/` on demand | yes (SKILL.md format) |
| 3 Hooks | the `downshift <harness>` subcommands; `hooks/hooks.json` is the plugin form | routing, telemetry, guardrails (shared Go) | yes; `hooks.json` is Claude Code packaging |
| 4 Agents | `agents/mechanical-worker.md` | one example with no pinned model ([subagents.md](../guides/subagents.md)) | Claude Code format; the rule applies everywhere |
| 5 Plugin | `.claude-plugin/plugin.json`, `marketplace.json` | one install: hooks, command, skills, agent | Claude Code only |

Contributor-only skills (`.claude/skills/downshift-layering`) stay out of the
plugin. Companions (Headroom, OmniRoute) are detected and documented, never
bundled: [companions.md](../guides/companions.md).

## Further reading

| Doc | Audience |
|-----|----------|
| [harness-matrix.md](../reference/harness-matrix.md) | What each harness honors |
| [session-models.md](../reference/session-models.md) | Operators |
| [layering.md](../contrib/layering.md) | Contributors |
| [engineering-loop.md](engineering-loop.md), [classifier-shadow.md](classifier-shadow.md) | Feedback and shadow mode |
| [context-optimization.md](context-optimization.md), [companions.md](../guides/companions.md) | Context layer |
| [report.md](../guides/report.md), [benchmark/README.md](../../benchmark/README.md) | Measurement discipline |
| [router-generalization.md](router-generalization.md) | Evaluation honesty |
| [pt/README.md](../pt/README.md) | Portuguese doc set |
