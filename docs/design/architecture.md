# Architecture — Downshift

High-level view for adopters and contributors. Implementation details live in
code and [CONTRIBUTING.md](../../CONTRIBUTING.md).

Downshift by Tiago Vilas Boas · https://github.com/tiagovilasboas/downshift

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
 classify + risk [D]  Classify (signals) → hash semantic boost → risk floor /
   abstain → NB second opinion (shadow unless opted in) ┆ adapt / score (shadow) ┆
             ▼
 Route + Check [D]  tier → effort → session-resolved model → verdict vs current →
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

### Read the decision, then the execution

BDI is a conceptual lens: task and session facts are **beliefs**, fixed safety
and capability goals are **desires**, and a checked routing plan is an
**intention**. Downshift implements deterministic routing, not a BDI framework
or an LLM planner. Available observations support offline review rather than
automatic policy updates.

![BDI lens: available task and session facts and fixed safety goals feed a checked plan; the harness executes and observations support offline review.](../img/downshift-bdi.png)

The complete flow keeps the same ownership boundary: Downshift recommends;
the harness executes; sensors record only the evidence they receive.

![Complete execution flow: delegation, session-first resolution, classification, guarded decision, session plan, harness execution, observation, reports and offline review.](../img/downshift-execution.png)

Editable sources: [BDI SVG](../img/downshift-bdi.svg) and
[execution SVG](../img/downshift-execution.svg). Their colors follow the
[brand tokens](../brand/downshift-brand-tokens.json).

### Step by step, with what the code does

1. **Task, planning, delegation** `[H]`. The main agent decides on its own
   when to delegate. Downshift sees nothing until the harness fires the hook
   on the spawn tool (`Agent|Task`, `spawn_agent`, `invoke_subagent`,
   `spawn_run`; `cmd/downshift/setup_targets.go`).
2. **Hook runner and adapter** `[D]`. `runHookAdapter` (`cmd/downshift/main.go`)
   is shared by every harness through `harnessRegistry` (`harnesses.go`). It
   bounds stdin (1 MiB, 2 s read timeout), parses the payload, sets the 1.5 s
   hook deadline and `ProjectDir`, recovers a panic into an allow, and only then
   calls the adapter's `Handle`. The catalog loads through
   `loadCatalogFailOpen` (`catalogload.go`): a panic in the embedded data or a
   hostile user override leaves the spawn unchanged instead of exiting 2, which
   a harness reads as a deny.
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
   never calls a provider. Every adapter calls `ResolveSessionModels` before
   classification: exact session metadata takes precedence, and the catalog
   supplies optional fallback metadata. The session is the complete candidate
   set. Declared capability order can rank new IDs; an unordered native inventory
   cannot. Malformed metadata rejects its ID instead of reopening catalog or
   positional fallback. Missing prices remain unknown, including family matches
   to a different generation. See [session models](../reference/session-models.md).
4. **Classification and risk** `[D]`. `classifyTask` (`internal/core/policy.go`)
   runs the signal scorer, the monotonic hash semantic boost, the risk floor and
   risk abstention (`internal/classify/risk.go`; from 64 KiB the candidate risk
   categories are scanned concurrently, same results and order), and the
   naive-Bayes second opinion.
5. **Route, guardrails, floor** `[D]`. `Route` resolves tier, effort and the
   session-resolved model, compares with the current model, then `Check`
   (`selfcheck.go`) applies R1, R2, R4, R5, R6, R7, R8, R9 (R3 retired): any
   violation turns the safe verdict into OK, which keeps the current model.
   Only a deterministic lift (risk, opted-in NB upshift, operator floor)
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
   effect. Observed on Claude Code; inferred on Codex; unconfirmed on Cursor
   (dropped on Free and legacy Pro). Antigravity has limited experimental proof
   on one build; KiroCrew uses policy and retry. Grok is config-only. Downshift does not run the
   child, cannot see its context, and never edits what it returns.
8. **Child end and usage** `[D]` from `[H]` payloads. PostToolUse on Claude Code
   is the launch of an async child: it carries `resolvedModel` (honored or
   refuted) and no tokens. SubagentStop supplies the child transcript, from
   which the shared recorder (`internal/childend`) prices tokens with catalog
   list prices. A nil port stays unobserved; absence is never inferred.
   Per-step routing latency is not a supported telemetry measurement. Claude
   Code also records the last recognized check command's result as
   `verification: passed|failed|none`; this is a transcript sensor, not proof
   that the implementation is correct. [Verification guide](../guides/verification.md).
9. **Telemetry and read-outs** `[D]`. `events.jsonl` holds routing metadata
   only (no prompts, tasks or paths). Appends do not fsync (a power loss can
   drop the last events; a process crash cannot). `telemetry.Summarize` is the
   single savings definition (gross saved, overspend, net) shared by `stats`,
   the dashboard `/api/status` and `dsmon`. `stats` estimates at list price,
   labelled `estimated, list price`, from the catalog's `prices_as_of`;
   `report` compares routed spawns with the control group, counting "completed"
   only for harnesses that declare an end event and have end records;
   `verification-report` groups observed checks and estimated cost by tier;
   `session` and `watch` read the same log. `serve` is an experimental local HTTP
   dashboard with a placeholder per-unit price, not a proxy or billing evidence.
10. **Benchmarks and tuning** `[D]`, offline. `benchmark` scores the classifier
    on labelled fixtures (tier accuracy, unsafe downgrade, over-routing);
    `eval-outcome` runs the outcome suite (minimal fixtures in OSS, full suite
    in downshift-labs); `feedback` attaches success/retry/failed to a routing
    id, and only reviewed `required_tier` labels enter `train`; `shadow-report`
    compares candidate classifiers that never touched a route.

---

## Layered view and import graph

Checked against `go list` on 2026-10-19. An arrow means "imports"; every arrow
points down or sideways to an inner package, and `core` never points back out.

```text
 harness hook (stdin JSON in, stdout JSON or exit code out)
        │
 ┌──────▼───────────────────────────────────────────────────────────────┐
 │ cmd/downshift   harnessRegistry · runHookAdapter · setup · doctor    │
 │   blank-imports: nbopinion policyfile quota routeadapt (init → hooks)│
 └──────┬───────────────────────────────────────────────────────────────┘
        ▼
 ┌─────────────────────────────────────────────────────────────────────┐
 │ internal/adapters/{claudecode cursor codex opencode kirocrew         │
 │   antigravity}: parse, HarnessCapabilities, wire response. No policy │
 │   imports: core hookutil quota · hookport childend telemetry         │
 │   (claudecode also contextopt sensor lifecycleobserver)              │
 └──────┬───────────────────────────────────────────────────────────────┘
        ▼
 ┌─────────────────────────────────────────────────────────────────────┐
 │ internal/core  Route · Check R1-R9 · PlanSession · Resolver · Caps   │
 │   imports only: classify fuse quotamodel session                     │
 │   hook vars (ports): MemoryAdjustHook TierFloorHook ScoreHook        │
 │   NBSuggestHook NBTrivialHook LoadQuotaHook LoadNativeQuotaHook      │
 │   ParseNativeQuotaHook RegisterNativeQuotaHook                       │
 └──▲───────────────────────────────▲──────────────────────────────────┘
    │ implement / register at init  │ import core (depend inward)
 ┌──┴────────────────────────┐   ┌──┴────────────────────────────────────┐
 │ hook providers            │   │ services over core                    │
 │ quota → LoadQuota*        │   │ catalog (Resolver) · discovery · models│
 │ policyfile → TierFloor    │   │ telemetry · hookport · childend       │
 │ adapt/routeadapt → Memory │   │ contextopt · sensor · server          │
 │ scoreindex → ScoreHook    │   │ benchmark · outcome · routingv2/*     │
 │ nbopinion (+nbtier) → NB  │   │ decisionintelligence                  │
 └───────────────────────────┘   └───────────────────────────────────────┘
 leaf packages used by the layers above: classify semantic quotamodel paths
   fuse (classify, semantic) · session (paths, quotamodel) · hookctx · codeintel (paths)
```

Edges worth knowing, all read from the code:

- `core` → `classify`, `fuse`, `quotamodel`, `session` (and `semantic`, `paths`
  below them). Nothing else.
- `session`, `fuse` and `codeintel` do not import `core`; `codeintel` imports
  only `paths`.
- `telemetry` → `core`, `codeintel`, `paths`. `hookport` → `core`, `telemetry`.
  `childend` → `core`, `hookport`, `telemetry`.
- `catalog` → `core`, `paths`. `discovery` → `catalog`, `core`, `paths`.
  `models` → `catalog`, `core`, `paths`.
- `policyfile` → `core`, `hookctx`. `quota` → `core`, `paths`, `quotamodel`.
  `nbopinion` → `core`, `nbtier`. `routeadapt` → `adapt`, `core`.
- `hookutil` → `core` (shared payload helpers; it is not a leaf).
- Not wired into any production package today: `internal/routecache` and
  `internal/routingv2/router` (tests only).

Invariant checks run for this refresh (all from the repository root):

| Invariant | Command | Result |
|---|---|---|
| Core production sources do not import `quota`, `nbtier`, `nbopinion`, `catalog`, `telemetry`, `adapt`, `policyfile`, `scoreindex` or an adapter | `go list -deps ./internal/core \| grep -E 'internal/(quota\|nbtier\|nbopinion\|catalog\|telemetry\|adapt\|policyfile\|scoreindex\|adapters)(/\|$)'` | holds (no match) |
| `session` does not import `core` | `go list -deps ./internal/session \| grep 'internal/core$'` | holds |
| `fuse` does not import `core` | `go list -deps ./internal/fuse \| grep 'internal/core$'` | holds |
| `codeintel` does not import `core` or `classify` | `go list -deps ./internal/codeintel \| grep -E 'internal/(core\|classify)$'` | holds |
| `core` test files import `catalog`, `nbopinion`, `nbtier`, `quota` | `go list -f '{{join .TestImports "\n"}}' ./internal/core` | true; not in the binary, but those tests run with the registered hooks |

No test or CI step enforces these checks; they are manual (see Known gaps).

## Data flows

**Spawn routing (hook path).** harness `PreToolUse` → `runHookAdapter`
(1 MiB, 2 s read, 1.5 s deadline, panic → allow) → adapter parses →
`ResolveSessionModels` → `core.Route` (classify, fuse, risk, `Check`, tier floor)
→ `PlanSession` → control group, pause and `DOWNSHIFT_NO_ROUTE` → wire
`updatedInput` (or block-with-reason for kirocrew) → two prompt-free appends
(`events.jsonl`, `loop-events.jsonl`).

**Prompt and DCI (opt-in).** `UserPromptSubmit` → `dci_hook.go` (1 MiB cap) →
`codeintel` (cache, BM25, abstain, pack within the byte budget) → stdout adds
a `<downshift-code-context>` block, or nothing. Imports no routing package.
Measured only for latency and bytes (`dci-events.jsonl`); no benefit to answer
quality or cost is claimed.

**Output compaction (observe only).** `PostToolUse` → `sensor` records tool
output bytes and the reduction `safe` mode would reach → `{}` (plus an optional
status line). The model never sees a shorter result. Compaction only happens
when the user pipes output through `downshift context compress`. Claude Code
hooks cannot rewrite tool output, so measure-only is a platform limit, not a
choice.

**Telemetry and stats.** appenders (`telemetry`, `childend`, `hookport`) write
JSONL; `stats`, `report`, `session`, `watch`, `serve` and `dsmon` read it.
`telemetry.Summarize` produces the one savings figure (gross, overspend, net);
`prices_as_of` dates the list prices behind it.

## Where state lives

All under `~/.downshift/` (override `DOWNSHIFT_STATE_DIR`; legacy
`~/.harness-downshift/`). Directories are created `0700`.

| File | Written by | Mode |
|---|---|---|
| `events.jsonl` (+ rotated backups) | hook, `end`, `feedback` | `0600`, no fsync |
| `loop-events.jsonl`, `dci-events.jsonl`, `probe.jsonl` | hook, DCI hook, `probe` | `0600` |
| `quota.json` | `quota` (temp file, atomic rename; lock dir) | `0600` temp file |
| `session-models.json` (operator), `discovered.json` (24 h TTL cache) | operator; `setup` and `models discover` | operator-owned; temp file `0600` |
| `catalog.json` (user override), `weights.json`, `adapt-memory.json` | operator, offline tools | operator-owned; `adapt-memory.json` `0600` |
| `paused/<session hash>` | `downshift session --pause` | `0600` |
| score cache and refresh stamp | `scoreindex`, `scorerefresh` | `0600` |
| `context-optimization.json` | `context enable` | `0644` (no secret inside) |
| DCI index cache | `codeintel` | dir `0700`, file `0600` |

Per project: `.downshift/policy.json` (read only). No file holds prompts, task
text or file paths. The harness's own settings are edited only by `setup`, with a
backup, and only for keys that are unset.

## Trust boundaries

| Boundary | Control |
|---|---|
| Harness → hook stdin | Untrusted JSON. 1 MiB cap (`+1` byte detects overflow), 2 s read timeout (`hookReadTimeout`), strict parse; any failure is a logged fail-open allow |
| Hook runtime | 1.5 s `HookBudget` for helper commands, panic recovery, catalog loaded under `recover`; a failure never exits 2 (deny) and an unchanged Claude Code or Codex spawn answers `{}`, not an explicit `allow`, so the user's ask rules still apply (Cursor's wire requires `{"permission":"allow"}`) |
| Hook → network | None. No provider call, no LLM, no price fetch. Quota and prices come from files |
| State directory | Single-user local files; `0700` dirs, `0600` logs; the DCI telemetry log refuses symlinks and non-regular files |
| Local dashboard (`serve`, `dsmon-server`) | Binds `127.0.0.1` only; rejects a non-loopback `Host` (DNS rebinding) and a foreign `Origin`; no CORS headers; `web/index.html` sets a CSP (`default-src 'self'`, `connect-src 'self'`, `object-src 'none'`, `form-action 'none'`); values rendered as text |
| Vendored mods | Third-party code pinned in NOTICE, read-only, no routing logic; `setup` installs them through `claude plugin install` |
| Release | goreleaser pinned to an exact version; build provenance attested for every archive (`gh attestation verify`) |

## CI and release guards

- `scripts/ci-classify-changes.sh` decides whether Go tests run. It is an
  allowlist: only existing images under `docs/` are provably inert, so every
  other path, including Markdown (read by `internal/docscheck`), embedded JSON
  and `.goreleaser.yml`, runs Go. An empty diff, or a tag, also runs Go. The
  classifier has its own test (`scripts/test-ci-classify-changes.sh`).
- Pushes to `main` are never cancelled by a newer run; only superseded pull
  request runs are. A cancelled code run followed by a skipped docs run once
  left `main` unverified.
- `ci-full` (weekdays) runs `go test -race -cover ./...` and `govulncheck`;
  the per-PR job does not run the race detector.
- `sync-prices` (Mondays, `workflow_dispatch`) refreshes the embedded list
  prices and `prices_as_of`, runs the catalog tests and opens a PR; a person
  merges it. It dispatches `ci` on its branch because a PR opened with
  `GITHUB_TOKEN` cannot trigger `pull_request` runs.
- Release: `goreleaser` at an exact version, then `attest-build-provenance`.

---

## Guide and sensor map

This is the outer harness around a coding agent (model + vendor harness + user
harness). Each concern has a guide before execution and a sensor after it;
sensor coverage depends on the harness, not just on emitted JSON.

| Concern / axis | Guia (feedforward) | Sensor (feedback) | Coverage |
|---|---|---|---|
| Routing safety / behaviour | Session metadata and project policy contract (inferencial); shared guards and tier floors (computacional) | Resolver, guardrail and all-adapter conformance tests (computacional) | Shared core; a passing test does not prove native rewrite honor |
| Cross-harness architecture / architecture fitness | Adapters parse/emit; policy stays in shared code (inferencial, `AGENTS.md`) | Conformance tests and code review against that boundary (computacional + inferencial) | Repository / PR / CI |
| Cost and outcome / behaviour | Control-group and evidence protocol (inferencial) | Honor, child-end and usage records; `report` and `verification-report` (computacional) | Claude Code has the most complete records; absent evidence stays unknown |
| Privacy and upkeep / maintainability | Prompt-free telemetry and bounded hook I/O contracts (inferencial) | Existing privacy, payload and documentation checks (computacional) | Repository checks; metadata rather than raw task text is persisted |

Human review decides whether offline findings justify a policy change. The
verification sensor observes a check result; it does not replace acceptance
tests, a security review or a production signal.

## Boundaries

| Concern | What Downshift does | Owner | Where |
|---|---|---|---|
| Harness session policy | `setup` offers `model: opusplan`, `permissions.defaultMode: plan`, for Claude Code, each written only when the key is unset (`--no-session-policy` skips them); `enabledPlugins["cc-plugin-you-should-know@builtin"]: true` is opt-in (`--you-should-know`) because it is billed. On Claude Code `setup` also runs `claude plugin install` for the sibling mods `savvy-progress`, `filetree` and `downshift-dock` (`--no-plugin` skips them; never by editing settings), and, outside `--dry-run`/`--undo`, runs model discovery (`discovery.Run`, 5 s) so session lists do not stay on handwritten values. The main agent's model is otherwise untouched. | D writes once, H enforces | `cmd/downshift/setup*.go`, `internal/discovery` |
| Subagent routing | The flow above: one decision per spawn, rewrite or allow, fail open. | D decides, H applies | `internal/core`, adapters, `runHookAdapter` |
| Context optimization | Native provider is **observe-only**: PostToolUse records tool output bytes and the reduction `safe` mode would reach; the hook never replaces the tool result the model sees. `downshift context compress` compacts piped output in `safe` mode as an explicit CLI step. Headroom is a companion: detected on PATH, recorded with `context enable headroom`, never installed, launched or wrapped. | D observes; X compresses | `internal/contextopt`, `internal/compressor`, `internal/sensor`, `providers/headroom` |
| Code Intelligence (DCI) | Independent subsystem: discovers repository code via Git (`discover.go`), extracts Go AST and multi-language structural symbols & calls (`parse.go`, `parse_lang.go`), builds incremental inverted index with reverse call graph (`index.go`), scores via BM25 with Production-First weighting & Call-Graph Co-Reference boost (`rank.go`), abstains on conversational chatter (`abstain.go`), packages structural sketches with downstream blast radius and prompt-injection defense within configurable budget (`pack.go`), and auto-injects via multi-harness hooks (`UserPromptSubmit`). CLI queries via `downshift dci impact` and `inspect`. Strictly decoupled from model routing. | D | `internal/codeintel/` |
| Observability | Prompt-free `events.jsonl`; honor, resolved, child_ended and usage records; `stats`, `report`, `session`, `watch`, `serve`, `probe` (maintainer). Prices come from the embedded catalog (`prices_as_of`, refreshed by the weekly `sync-prices` PR, [prices.md](../reference/prices.md)). | D | `internal/telemetry`, `internal/childend`, `internal/server`, `.github/workflows/sync-prices.yml` |
| Diagnosis and recommendations | `doctor` (binary, state dir, catalog, hooks per harness, model override, quota, policy, context optimization, companion, semantic boost) with a `fix` per failed check; skills `downshift-setup`, `downshift-audit`, `cost-reducer` write nothing without a yes. | D | `cmd/downshift/doctor*.go`, `skills/` |
| Benchmark and economic evaluation | Classifier KPIs, outcome evals, control-group report with bootstrap CI, feedback and offline training. Public `benchmark/` holds the published report; datasets stay in downshift-labs. | D | `internal/benchmark`, `internal/outcome`, `internal/routingv2/training` |

### Shadow and opt-in paths

| Path | Status | Switch |
|---|---|---|
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

## Domain Architecture Layers (Clean Architecture / SRP)

Downshift's codebase and documentation are structured strictly according to Clean Architecture, Single Responsibility (SRP), and the Dependency Inversion Principle. Dependencies point inward toward core domain policies; autonomous subsystems like Code Intelligence maintain absolute decoupling from routing logic; and interface adapters contain zero business policy.

| Domain Layer | Clean Architecture Role | Responsibilities & Invariants | Core Packages | Key Documents |
|---|---|---|---|---|
| **Domain 1: Subagent Model Routing & Guardrails** | Core Domain / Business Logic | Deterministic subagent routing, regex signal scoring, monotonic hash semantic boost, R1-R9 guardrails (holding unconfident moves under doubt), project tier floor (`.downshift/policy.json`), session candidate allowlists, and quota gating. Invariants: bounded reads (1 MiB), fail-open on timeout or error, zero network/LLM on hot path, zero prompt storage. | `internal/core`, `internal/classify`, `internal/fuse`, `internal/session`, `internal/quotamodel`, `internal/catalog`, `internal/semantic`, `internal/nbopinion`, `internal/policyfile` | [../domains/subagent-routing.md](../domains/subagent-routing.md), [architecture.md](architecture.md), [router-generalization.md](router-generalization.md), [../reference/complexity-weights.md](../reference/complexity-weights.md), [../reference/session-models.md](../reference/session-models.md) |
| **Domain 2: Code Intelligence & Context Layer (DCI)** | Autonomous Domain Subsystem | Zero-runtime AST structural symbol and call extraction (Go, TS/JS, Python, Rust, PHP, Java, C#), incremental inverted index, reverse call-graph blast radius analysis (`InvertedIndex.Callers`), BM25 ranking with Production-First weighting & call-graph co-reference boosting, conversational abstention, and structural context packaging with prompt-injection delimiters (`<downshift-code-context>`). Invariants: strict domain isolation (zero imports between `internal/codeintel` and `core`/`classify`), strict latency budgets (cold <= 2500ms, warm p95 <= 300ms), fail-open (timeouts/errors exit 0), epistemic honesty (records only observed counts and byte sizes, never synthetic dollar savings). | `internal/codeintel` | [../domains/code-intelligence.md](../domains/code-intelligence.md), [code-intelligence.md](code-intelligence.md), [../evidence/dci-v04-blast-radius.md](../evidence/dci-v04-blast-radius.md), [../evidence/dci-mvp-v01.md](../evidence/dci-mvp-v01.md) |
| **Domain 3: Output Compaction & Context Optimization** | Supporting Domain / Optimization | Native AST/JSON context compressor (`safe` mode CLI compaction), observe-only tool output sensor (PostToolUse byte measurement and hypothetical token reduction), Headroom companion detection & telemetry (`context enable headroom`), and initial session context policy (`opusplan`, plan mode). Invariants: PostToolUse hook never alters live output seen by the model (observe-only); companion tools are detected and recorded, never installed, wrapped, or launched. | `internal/contextopt`, `internal/compressor`, `internal/sensor`, `providers/headroom` | [../domains/context-optimization.md](../domains/context-optimization.md), [context-optimization.md](context-optimization.md), [../guides/companions.md](../guides/companions.md), [../../skills/cost-reducer/SKILL.md](../../skills/cost-reducer/SKILL.md) |
| **Domain 4: Observability, Evidence & Verification** | Telemetry, Auditing & Evaluation | Prompt-free event logging (`events.jsonl`), 10% unrouted control group A/B baseline, catalog-priced cost & savings reports (`stats`, `report`; one definition in `telemetry.Summarize`), verification reports (`verification: passed|failed|none`), capability matrix, and evidence ladder (rungs A-E). Invariants: telemetry persists metadata only; child-end tokens are priced with catalog list prices; unobserved ports remain absent, never inferred. | `internal/telemetry`, `internal/childend`, `internal/hookport`, `internal/server`, `internal/benchmark`, `internal/outcome` | [../domains/observability-telemetry.md](../domains/observability-telemetry.md), [engineering-loop.md](engineering-loop.md), [../reference/capability-matrix.md](../reference/capability-matrix.md), [../guides/report.md](../guides/report.md), [../guides/verification.md](../guides/verification.md), [../guides/evidence.md](../guides/evidence.md) |
| **Domain 5: Harness Adapters & Packaging** | Interface Adapters & Delivery | Harness-specific edge I/O (Claude Code, Cursor, Codex, OpenCode, KiroCrew, Antigravity), fail-open hook runner (`runHookAdapter`), 5 packaging layers (Memory, Skills, Hooks, Agents, Plugin), Claude Code plugin, and mods (`cache-tax` inside the plugin; `savvy-progress`, `filetree`, `downshift-dock` as sibling plugins that `setup` installs; `downshift-router` experimental). Invariants: adapters embed no routing policy; adapters only parse wire format and emit core decisions; panic recovery ensures fail-open allow. | `internal/adapters/*`, `cmd/downshift`, `mods`, `hooks` | [../domains/harness-adapters-packaging.md](../domains/harness-adapters-packaging.md), [../reference/harness-matrix.md](../reference/harness-matrix.md), [../contrib/layering.md](../contrib/layering.md), [mod-router.md](mod-router.md), [../guides/claude-code.md](../guides/claude-code.md) |

---

## Module boundaries

| Package | Responsibility |
|---------|----------------|
| `internal/classify` | Regex classifier, `signals.go`, language guard, risk floor. No tier, no harness, no catalog |
| `internal/core` | Route policy: `TierOf`, `Route`, guardrails, session routing (`PlanForSession`, `RouteForSession`, `InferSessionFromCurrent`, `CanWriteSessionID`), escalation, self-check, `HarnessCapabilities`. Imports `classify`, `fuse`, `quotamodel` and `session` (checked 2026-10-19: `go list -deps ./internal/core` lists only those plus `semantic` and `paths`). Production sources do not import `quota`, `nbtier`, `nbopinion`, `catalog`, `telemetry`, `adapt`, `policyfile`, `scoreindex` or an adapter. Core test files do import `catalog`, `nbopinion`, `nbtier` and `quota`; they are not in the binary. Optional layers plug in through hook variables set in their `init` (see Known gaps). Score selection stays here on purpose (`ScoreHook`); do not merge it with the naive-Bayes opinion. `NBUpshift` and `NBDownshift` stay on `Decision`. Env stays a typed snapshot set at the edge (`SetConfig`) |
| `internal/quotamodel` | Pure quota evidence types: `Status`, `Window`, `Snapshot`, `Validate`, `Evaluate`, `Abandoned`, `SourceName`. No file I/O. Landed: `core` imports this package; file reads and native parsing enter through `LoadQuotaHook`, `LoadNativeQuotaHook`, `ParseNativeQuotaHook` and `RegisterNativeQuotaHook`, registered by `quota`, the same pattern as `MemoryAdjustHook` |
| `internal/session` | `SessionList` and the loaders that do not need `Decision` or `Resolver`: file, hook marks, discovered file, quota snapshot attachment. Landed (`5f94e07`): `core` imports this package; this package must not import `core`. `PlanForSession`, `RouteForSession`, `InferSessionFromCurrent` and `CanWriteSessionID` stay in `core` |
| `internal/fuse` | Landed (`110ef31`). Semantic boost policy: raise-only, no-signal skip. Imports `classify` and `semantic`. Does not import `core`. `core` keeps `ClassifyWithSemantic` as a wrapper. Stays in-process and does not exec |
| `internal/nbopinion` | Landed (`4d3e857`, PR 156). Hook funcs that call `nbtier.Suggest` and `nbtier.Trivial`, registered in `init`. May import `core` and `nbtier`. Core production sources do not import this package or `nbtier`. `NBUpshift` and `NBDownshift` stay on `Decision` in `core`. `cmd` blank-imports this package so `init` runs. Stays in-process and does not exec. Env stays a typed snapshot set at the edge (`SetConfig`) |
| `internal/classify` | Regex classifier, `signals.go`, language guard, risk floor (concurrent category scan from 64 KiB). No tier, no harness, no catalog |
| `internal/catalog` | Embedded `catalog.json` (with `prices_as_of`), user override merge (recovers into the embedded data on a panic), `Resolver` |
| `internal/discovery`, `internal/models` | Session model discovery (sources, order, `discovered.json` cache; run by `setup` and `models discover`) and `models list/check/pull`. Both import `catalog` and `core` |
| `internal/adapters/*` | Harness-specific I/O only. Adapters do not choose the tier; they call `core.Route`, then apply the plan core returns |
| `internal/policyfile`, `internal/adapt`, `internal/routeadapt`, `internal/scoreindex` | Project tier floor (`TierFloorHook`), shadow tier memory (`MemoryAdjustHook`) and the score cache (`ScoreHook`), plugged into core at `init`. `cmd` blank-imports the first three kinds and imports `scoreindex` directly |
| `internal/quota`, `internal/hookctx`, `internal/hookutil` | Offline budget evidence (file I/O); per-hook deadline and project dir; shared field helpers. `quota` keeps `Load`, `Store`, `ParseNative` and native registration, aliases the `quotamodel` types so adapters can keep saying `quota.Snapshot`, and registers `LoadQuotaHook` and the native quota hooks. Core production sources do not import `quota` |
| `internal/hookport`, `internal/childend` | Honor, usage and child-end contract; one shared recorder for every harness |
| `internal/telemetry` | Append-only local JSONL (no per-spawn fsync); prompt-free events; `Summarize` (single savings definition), stats, report, control group, session tree. Imports `core` (tiers, prices) and `codeintel` (DCI summary type in the export) |
| `internal/contextopt`, `internal/compressor`, `internal/sensor` | Context optimization registry, native compressor (observe/safe), tool-output sensor |
| `internal/codeintel` | Downshift Code Intelligence (DCI): pure Go symbol extraction, incremental inverted index, BM25 ranking, abstention, and structural context packager. Isolated from routing: imports only `paths`; `core` and `classify` do not import it (`telemetry` imports it for the stats export type). |
| `internal/semantic`, `internal/nbtier` | In-process hash embedder (`semantic`) and naive-Bayes opinion (`nbtier`). The raise-only boost policy lives in `internal/fuse`. `nbopinion` calls `nbtier.Suggest` and `nbtier.Trivial`. Core production sources do not import `nbtier`. The semantic boost and the naive-Bayes opinion stay in-process and do not exec |
| `internal/routingv2/*`, `internal/decisionintelligence`, `internal/outcome`, `internal/benchmark` | Shadow classifiers and training, advisory recommendation, outcome evals, classifier KPIs. `routingv2/router` and `internal/routecache` are not imported by any production package today (tests only) |
| `internal/server` | Local dashboard over the event log (`serve`, `dsmon-server`) |
| `cmd/downshift` | CLI, `harnessRegistry`, `runHookAdapter`, `runEnd`, setup, doctor. Blank-imports `nbopinion`, `policyfile`, `quota`, `routeadapt` |

**Invariants:** adapters embed no routing rules; adding a harness is one
`harnessRegistry` entry plus its adapter package, with a port entry only when
that harness's own hook sends honor or usage fields; a nil port is unobserved,
never inferred. Dependency check: `go list -deps ./internal/core` (manual; no test enforces the graph, see Known gaps).

---

## Configuration on disk

State directory `~/.downshift/` (legacy `~/.harness-downshift/`, see
[rename.md](../brand/rename.md); override with `DOWNSHIFT_STATE_DIR`). See
"Where state lives" below for what each file is and its permissions.
Per project: `.downshift/policy.json` (tier floor). Overrides:
`DOWNSHIFT_EVENT_LOG`, `DOWNSHIFT_SESSION_MODELS`, `DOWNSHIFT_QUIET`, flags in
README and [minilm-semantic.md](minilm-semantic.md).

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
| 5 Plugin and mods | `.claude-plugin/plugin.json`, `marketplace.json`, `mods/` | one install: hooks, command, skills, agent; sibling mods (`savvy-progress`, `filetree`, `downshift-dock`) that `setup` installs | Claude Code only |

Contributor-only skills (`.claude/skills/downshift-layering`) stay out of the
plugin. Companions (Headroom) are detected and documented, never
bundled: [companions.md](../guides/companions.md).

## Known gaps and limits

These are real and open on 2026-10-19. Nothing here is hidden elsewhere.

1. **`telemetry` is large.** 31 non-test files (events, cost, stats, report,
   control group, rotation, pause, session tree, completion) behind one
   package, imported by adapters, `childend`, `hookport` and `server`. A split
   (event log and appenders vs read-outs) is proposed, not done.
2. **`init`-registered hooks in `core` (Fullstack P1-1).** The result of
   `core.Route` depends on which packages were blank-imported: the tier floor
   (`policyfile`), shadow memory (`routeadapt`), naive-Bayes (`nbopinion`) and
   the quota loaders (`quota`) exist only if `cmd/downshift` imported them.
   Adapter tests link `quota` (adapters import it) but not `policyfile`,
   `nbopinion` or `routeadapt`, so they run without the tier floor and the
   opinion. Behaviour is covered at `runHookAdapter` and `core.Route` tests, but
   an adapter test alone does not prove production routing.
3. **No import-graph sensor.** The invariants above are checked by hand with
   `go list`. There is no test or CI step that fails when core gains a
   forbidden import.
4. **Rotated log backups are not read.** `stats` prints a note and counts only
   the live `events.jsonl`; totals drop after a rotation (100 MB by default).
5. **DCI is opt-in** and measured only for latency and injected bytes. No
   evidence yet that it improves answers or lowers cost.
6. **Savings are list-price estimates.** They use the embedded catalog at
   `prices_as_of` (weekly PR), not provider invoices. Real-cost figures exist
   only where the harness reports tokens (Claude Code most completely). The
   one tracked field trial is -12% on mechanical pieces, not a general rate.
7. **Compaction is measure-only on Claude Code.** Hooks cannot rewrite tool
   output, so the sensor reports a hypothetical reduction.
8. **Hook latency** after removing the per-spawn fsync: p50 8.5 ms, p95 9.7 ms
   for a Claude Code Task payload (n = 200, Apple M3 Pro, macOS 26.6, Go
   1.27.1; `downshift version` p50 7.7 ms; process floor 2.2 ms). Not measured on
   Linux or Windows, on a populated state directory, or under parallel
   spawning. Trade-off: a power loss can drop the last events.
   [Evidence](../evidence/perf-2026-10-19.md).
9. **OpenCode packaging is unverified.** The adapter and conformance tests
   exist; the forwarding plugin (`mods/downshift-opencode`) has no live spawn
   observed. Cursor, Codex and Antigravity rewrite honor is also bounded as in
   the [harness matrix](../reference/harness-matrix.md).
10. **Stale diagram source.** `docs/img/architecture.svg` (and `-dark`) is
    hand-drawn, not generated, and predates `classify`, `fuse`, `session`,
    `opencode` and the hook ports; it still names e5 and graphify helpers.
    Nothing links to it. The ASCII diagram above is the maintained one.
    `downshift-bdi` and `downshift-execution` are current.
11. **Dead-end packages.** `internal/routecache` and `internal/routingv2/router`
    have no production importer.

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
