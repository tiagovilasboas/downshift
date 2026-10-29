# Routing v2: declare, bound, apply, measure

Downshift by Tiago Vilas Boas · https://github.com/tiagovilasboas/downshift

Status: **Proposed** (owner review); **superseded in part by O8 (2026-10-28)**, see the section at the end. Date: 2026-10-26. Roadmap and task cards:
[`../internal/tasks/2026-10-26-routing-v2/README.md`](../internal/tasks/2026-10-26-routing-v2/README.md).
Inputs: a read-only diagnosis of today's routing (`origin/main` 800a8e4e) and a clean-room behavioural
analysis of an external orchestrator (AGPL-3.0, read only; no code, prompt or schema copied).
Scope: **model routing only**. The Compactor (`execwrap`, `compressor`) and DCI (`codeintel`) are kept as they are.

## 1. Why change

Today's routing infers the right model from the task text, inside each harness's hook.

| Fact (2026-10-26) | Number |
|---|---|
| Child tokens moved to a smaller model by Downshift, last 30 days, real use | 4.1% (23.9M of 579M) |
| Claude Code spawns that changed model | 8 of 158 (5.1%), all from Opus |
| Spawns where the parent already asked for haiku or sonnet | 76% (82% of child tokens) |
| Classifier tier accuracy on blind sets / abstention there | 42–53% / 62–77% (100% on the tuned holdout) |
| Real decisions unconfident / defaulting to MEDIUM | 62.5% / 66% |
| Decisions corrected by a guardrail (held) | 33.5% |
| Claude Code rewrites on `main` after #570 for a subscription user | 0 (no candidate list; 9 of 9 held in a repro) |
| Harnesses that can apply a rewrite and report usage | Claude Code only |
| Routing-path code / signal sources / `DOWNSHIFT_*` knobs | ~15k lines in 15 packages / 11 / 24 |

Two conclusions. **Inference is the weak link**, not the hook: the regex classifier does not generalise,
so the guardrails correctly hold most of what it proposes. **The decision that actually saves tokens is
already declared**: the parent agent picks haiku or sonnet for most work. Per-harness payload inference is
also the part that costs most to keep across six harnesses with uneven contracts.

The external orchestrator (studied clean-room) points the other way: its core never reads the task text; the
decision comes from **declared inputs** (a role, a kind and effort class reported by the planner, user rules
with quota gates) and the core only bounds, applies and annotates. That shape is harness-agnostic by
construction. Its weak points (an LLM choosing by default, advisory-only quota, no freshness bound, task text
persisted) are what Downshift must not copy.

## 2. The model

```
   DECLARE                         BOUND                          APPLY                        MEASURE
 ┌──────────────────────┐   ┌───────────────────────────┐   ┌─────────────────────────┐   ┌──────────────────┐
 │ policy.json (user)    │   │ guard (deterministic):     │   │ hook rewrite (Claude)    │   │ telemetry (enums │
 │  profiles, rules,     │──▶│  risk floor never small,   │──▶│ launch flag (route CLI)  │──▶│  + tiers + %,    │
 │  quota gates          │   │  never widen, doubt keeps, │   │ native config compile    │   │  no task text)   │
 │ hint (parent/LLM):    │   │  explicit_only, foreign id,│   │  (agents' model fields)  │   │ accounting v1    │
 │  kind, effort class   │   │  headroom (worst window)   │   │ block + retry (Kiro)     │   │                  │
 └──────────────────────┘   └───────────────────────────┘   └─────────────────────────┘   └──────────────────┘
          ▲                                                                                         │
          └──────────────── suggestions (offline, from measured decisions) ─────────────────────────┘
```

**Precedence** (first that applies wins; deterministic; ties broken by declaration order):

1. An explicit model the caller pinned (agent definition `model:`, a `--model` flag) **is kept** unless the
   guard forbids it (it never forbids keeping; it only forbids moving down into a risk area).
2. A **profile** named by the caller or matched by agent type (e.g. `reviewer → strong`). A profile is the only
   source of what it sets: a call that names a profile and also overrides model or effort is refused (config
   error, logged), and the spawn proceeds unchanged.
3. The **first matching rule**: `match` on `kind`, `effortClass`, `agentType`, `harness`; optional quota
   `gates` (`window < X%`); result is a **tier** (`small|mid|strong`), never a raw id.
4. A **hint** from the parent (enum only: `kind`, `effortClass`, optional `tier`), accepted only to move
   **down or keep**, never up past what the requester asked for.
5. Otherwise **keep the requested model** (doubt keeps; today's "doubt stays allow").

The tier resolves to a model through the harness's candidate set (§4) and `catalog.json`.

**Guard** (always applied after the decision; can only *keep* or *raise*, never lower):
- Risk floor: secret, authz, crypto, payment, vuln, pii, destructive, concurrency, prompt-injection
  vocabulary in the task text ⇒ never `small`; the existing risk scanner is the only text reading kept.
- Never widen: never move to a model the harness cannot honour, never write a foreign id (R4), never
  pick `explicit_only` automatically, never change sibling fields (only `model`, `reasoning_effort`).
- Headroom: per provider, strain = rate-limited ? +∞ : the fullest window's used %. A gate with no reading
  **passes** (missing data allows); a reading older than its TTL counts as missing. Strain is advice for
  rules (gates) and for the routing note; it never forces an upshift.
- Fail-open: any error, unknown field or timeout ⇒ spawn unchanged (1 MiB, 2 s, 1.5 s budgets unchanged).
- Config errors (unknown profile, malformed rule) are refused **at policy load** (`downshift policy check`),
  so they never reach the hook as surprises.

**Routing note** (telemetry, enums only): `verdict` = `kept|profile|rule|hint|guard_raised|refused_config`,
`rule_id`, `profile`, `tier_from`, `tier_to`, `headroom_window`, `headroom_used_pct`, `harness`, `channel`.
No task text, no paths, no prompt.

## 3. Channels (one decision, several ways to apply it)

| Channel | How the decision reaches the model | Harnesses | New? |
|---|---|---|---|
| **Hook rewrite** | PreToolUse `updatedInput.model` / `reasoning_effort` | Claude Code (completed), Cursor, Codex, OpenCode (as today) | kept, decider swapped |
| **Launch** | `downshift route --harness <h> --json` (stdin: hint + agent type; stdout: `{model, effort, tier, note}`); the launcher passes `--model/--effort` | any CLI launched by a script or an orchestrator: `kiro-cli chat --no-interactive --model --effort`, `claude -p --model`, `codex exec --model` | new |
| **Native config compile** | `downshift policy apply --harness <h> [--dry-run]` writes the tier of each profile into the harness's own agent definitions | Claude Code `.claude/agents/*.md` `model:`; Kiro `~/.kiro/agents/*.json` `model`; Codex profiles; OpenCode agent `model` | new |
| **Block and retry** | the hook blocks a spawn whose declared model violates the guard and names the allowed one | KiroCrew `spawn_run` (as today) | kept |

The **native config** channel is what makes routing work where no hook can rewrite: the harness itself starts
each agent on the declared model. It runs at setup or on demand, shows a diff first (`--dry-run`), and backs
up what it changes, like `downshift setup`.

## 4. Candidate sets are harness data

A harness's usable model ids are a fact in `HarnessCapabilities` (layering rule 3), not a list the user must
maintain or an inference per payload. Claude Code: the family aliases `haiku`, `sonnet`, `opus` (always
accepted, subscription or API). This removes the #570 regression class: routing never depends on a session
list being present. Discovery (`downshift models discover`) can still add ids; it never removes the defaults.

## 5. Skills: the hint channel for the parent LLM

A `downshift-route` skill (one `SKILL.md`, small) teaches the parent agent to **declare** when it delegates:
`kind` (`mechanical|read|code|design|review|risky`) and `effortClass` (`light|standard|deep`), as an enum
prefix in the spawn's description, and to call `downshift route` when it launches CLIs itself. The LLM
proposes; rules dispose: a hint can only move down or keep, the guard still applies, and an unparseable hint
is ignored.

Distribution follows the shared-store convention the ecosystem uses: the skill lives once in
`~/.agents/skills/downshift-route/` (with an entry in `~/.agents/.skill-lock.json`), and each harness's own
skills directory links to it: `~/.claude/skills`, `~/.codex/skills`, `~/.config/opencode/skill`,
`~/.gemini/skills`, `~/.kiro/skills` (Kiro agents load skills through `skill://` resources). `downshift setup`
creates the links (never over a user-owned file; dry-run first; Windows falls back to a copy).

## 6. How it looks in Kiro (KiroCrew)

- **Native config:** `downshift policy apply --harness kirocrew` sets `model` in each `~/.kiro/agents/*.json`
  from its profile (e.g. `kirocrew-dashboard-author → mid`, a docs agent → small). Kiro then starts that agent
  on that model; no rewrite needed.
- **Launch:** scripts and orchestrators run `kiro-cli chat --agent <a> --no-interactive --model $(downshift route
  --harness kirocrew --json | jq -r .model) --effort ...` (flags verified on 2026-10-26).
- **Guard:** the existing preToolUse hook keeps blocking a `spawn_run` whose model violates the risk floor and
  names the allowed model (block and retry), now with the same decider.
- **Skill:** `~/.kiro/skills/downshift-route` → link to the shared store.
- **Not covered:** native `use_subagent` / `spawn_sub_agents` without a per-entry model stay outside, as today.
- **Measurement:** Kiro reports no child usage; its rows stay "decided and applied, not measured".

## 7. Architecture (screaming, one domain per package)

New code lives under one domain directory; each subpackage has one reason to change.

```
internal/routing/
├── policy/     declared profiles, rules, gates; load + validate (fail-closed on config)
├── hint/       parent/LLM hints: parse enum prefix, agent type; reject anything else
├── headroom/   provider strain from quota readings: worst window, rate-limited=+inf, TTL
├── guard/      risk floor, never widen, explicit_only, foreign id, sibling fields (uses classify/risk only)
├── decide/     precedence §2 → tier → model via candidates + catalog; pure, no I/O
├── note/       routing note enums for telemetry
└── compile/    native config writers per harness (data from HarnessCapabilities; writers are thin)
cmd/downshift/  route, policy check|apply, setup links (thin commands)
internal/adapters/*  unchanged role: parse, declare capabilities, emit; call routing/decide
```

Dependency direction: `adapters`, `cmd` → `routing/decide` → `policy`, `hint`, `headroom`, `guard` →
`catalog`, `quotamodel`, `classify/risk`. `routing` never imports an adapter, `telemetry`, `codeintel` or
`execwrap`. `archtest` gets allow-list rules for `internal/routing/...`.

**Removed after the switch** (each in its own PR, once v2 is default): the regex tier classifier as a decider
(the risk scanner stays), the two naive-Bayes opinions, adapt memory, score routing, the routingv2 shadow
classifier, decisionintelligence and the gateway shadow (~4.7k lines with no production effect today).
The v1 classifier may stay one release as an offline **suggester** (it proposes rules from measured
decisions; it never decides).

## 8. Invariants kept

Deterministic decision, no LLM and no network in the hook; fail-open budgets; doubt keeps the requested model;
risk never goes small; only `model` and `reasoning_effort` change; prompt-free telemetry; one token-savings
definition; layering rule (policy in shared layers, adapters hold none). New: config errors fail closed at
load time; a hint can never raise cost.

## 9. Coverage inventory (architecture-coverage procedure)

| Surface | Packages | Effect | Account | This phase |
|---------|----------|--------|---------|-----------|
| Routing | `core`, `classify`, `fuse`, `session`, `catalog`, `quota` → new `routing/*` | writes child model (hook), passes launch flags, writes native agent config | `telemetry.Summarize` | **in** (decider replaced; classify keeps risk only) |
| Shell | `execwrap`, `shellwrap` | rewrites command / advises retry | bytes, outside `Saved` | **out** (unchanged) |
| DCI | `codeintel` | opt-in context injection | latency + bytes | **out** (unchanged) |
| Telemetry | `telemetry`, `childend`, `hookport` | measures | token, no dollar | **in** (routing note fields; no prompt) |
| Accounting | `accounting` | CLI does not import; ADR Proposed | dollar, +15% bias | **out** (accounting v1 plan stays separate; never merged with the token account) |
| Adapters | `internal/adapters/*` | translates the wire | honor + usage per harness | **in** (call `routing/decide`; capabilities gain candidate sets) |
| Quality | `outcome`, `benchmark` | offline | acceptance suite | **in** (golden decision tables replace classifier accuracy as the gate) |

Doubt stays allow: with no profile, rule or hint, the decider keeps the requested model (§2 step 5).
No phase changes R7/R8 semantics.

## 10. What this does not claim

No saving figure is claimed for v2 before it runs. The four-arm battery measured v1 on mechanical tasks; it
does not transfer. The owner decided on 2026-10-26 to stop experiments for now; v2 is gated by deterministic
golden tests and by the measured routing note in real use, read with `downshift report`.

## 11. Reference patterns (Anthropic cookbooks, MIT)

Read in the owner's order. Each row says what v2 takes and what it leaves; no code is copied.

| Pattern | What it shows | What v2 takes |
|---|---|---|
| [Orchestrator-workers](https://github.com/anthropics/claude-cookbooks/blob/main/patterns/agents/orchestrator_workers.ipynb) | a lead decomposes, workers run bounded subtasks, the lead merges | the Claude Code parent is the orchestrator; each delegation carries its own profile or hint, so the tier is decided per subtask, not per session |
| [Async multi-agent](https://github.com/anthropics/claude-cookbooks/blob/main/patterns/agents/async_multi_agent_orchestration.ipynb) | workers run concurrently in isolated contexts | the decision is per spawn and stateless; nothing in `routing/*` assumes order or a shared session list (the #570 lesson) |
| [Plan big, execute small](https://github.com/anthropics/claude-cookbooks/blob/main/managed_agents/CMA_plan_big_execute_small.ipynb) | a frontier orchestrator with no tools plans and synthesizes; cheap workers do the token-heavy reading; on the authors' runs 84–98% of team input tokens billed at the worker rate | **the core of v2.** Frontier parent plans, profiles send reading and mechanical subagents to the small tier, the guard keeps risk prompts off it. Its caveats become rules: compare only at matched rigor (the acceptance suite, not raw spend); delegation has a floor cost, so too-narrow briefs lose; it does not pay on narrow tasks or when judgment is needed on the raw material (keep the requested model there) |
| [Dynamic workflows](https://github.com/anthropics/claude-cookbooks/blob/main/claude_agent_sdk/08_Dynamic_workflows.ipynb) | Claude Code's `Workflow` tool runs a script that spawns many subagents; `agent(prompt, {model})` sets a model per stage; `CLAUDE_CODE_SUBAGENT_MODEL` overrides all | a fifth channel candidate: the skill tells the parent to put the declared tier in each stage's `model`. **Unverified:** whether PreToolUse sees workflow `agent()` spawns at all. Card R0b checks it before any design depends on it |
| [Coordination patterns](https://claude.com/blog/multi-agent-coordination-patterns) (article) | five patterns; start with orchestrator-subagent, move only when a need appears; set budgets and stop rules | v2 stays orchestrator-subagent: no bus, no shared state, no supervisor. Budgets are shown (headroom in the routing note), not enforced by rerouting |

What v2 rejects from these: an LLM selector in the routing loop (the cookbooks' `route()` picks from a closed set
with a model call; v2 keeps the closed set and drops the call, the parent declares it), and long-lived worker
supervision (an orchestrator's job, §3 of the tasks out-of-scope list).

Measurement follows the cookbook's method: per-delegation usage from the child-end events, compared with a
rigor-matched control. A run where the parent did not delegate paid a frontier round trip for nothing; the
routing note records `delegated=false` for that, and the report counts it.

## 12. Superseded by O8 (2026-10-28)

Owner decision O8: the orchestrator LLM chooses agent, model and effort; Downshift applies the choice, enforces
the user's declared policy and measures. Sections 1 to 11 stay as the historical design. Where they disagree
with this section, this section wins.

| Section above | Status |
|---|---|
| §2 precedence | Now: a pinned model is kept; a declared profile; the first matching declared rule; the orchestrator's hint, **down only for now** (moving up is specified, not built); else keep the requested model |
| §2 guard: risk floor | **Removed.** The protection is the orchestrator's judgment, guided by the `downshift-orchestrator` playbook skill, plus tier floors the user declares in `policy.json`. Residual risk: with no declared floor, nothing stops a risky task from going small |
| §2 guard: never widen | Kept. The guard now refuses only a foreign id or an `explicit_only` model, and keeps on doubt |
| Built-in default agent-type table (N11) | **Removed.** Its content became guidance in the playbook skill |
| Automatic upgrade after a recorded failure | **Removed.** The orchestrator decides the next model |
| Economy mode on built-in types | **Removed** (no built-in rows to act on); `policy economy` still exists, its removal is pending |
| §8 "no LLM in the hook", "risk never goes small" | Read as: Downshift's own code never calls an LLM and the orchestrator's choice is applied deterministically; "risk never goes small" no longer holds |
| Classifier | Telemetry, key of user-declared floors and DCI input; not a decider |

Added by O8: playbooks (`downshift playbook`, implemented) and the `downshift-orchestrator` skill installed by
`downshift setup` (implemented); the MCP server `downshift mcp serve` (implemented: four information tools; the worker tools are specified).
