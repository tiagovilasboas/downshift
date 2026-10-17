# Companions: tools that work next to Downshift

harness-downshift by Tiago de Carvalho Vilas Boas
https://github.com/tiagovilasboas/downshift

Downshift does one thing: before a subagent spawns, it picks that subagent's
model and effort. Other token levers live in other tools. This page says which
ones Downshift knows about, what it does with each (detect, record, document,
or nothing), and what it will never do on their behalf.

**Default and optional.** Headroom is the default companion: when its binary
is on PATH, `downshift context enable` records it and `downshift doctor`
reports it. Everything else on this page is optional and documentation only.

**Claude Code first.** Downshift's core, adapters and docs serve every
harness it ships an adapter for (Claude Code, Cursor, Codex, Antigravity,
KiroCrew; Grok by config). Claude Code is where it is most complete and most
verified: rewrite honored with evidence, session model inferred from the
current model, usage and child-end hooks, the `/downshift` command and the
statusline. The companion table below says per harness what each tool
supports; the Downshift side of that table is in
[harness-matrix.md](../reference/harness-matrix.md).

## What Downshift never does for a companion

- It does not install, launch, wrap, unwrap or configure a third-party tool.
  `downshift context enable headroom` writes one field in Downshift's own
  `context-optimization.json`; it runs no `headroom` command and touches no
  harness settings. (An earlier RTK integration ran the companion's own
  `init`; it was removed in #71 and this rule comes from that.)
- It does not bundle third-party skills, rules or agent files in `skills/`.
  Most are single-harness, they add always-loaded context to every session,
  and some carry licences (CC BY) or default telemetry that Downshift does
  not want to ship under its own name.
- It does not count a companion's savings as its own. `downshift stats` is
  routing; `headroom savings` is compression. When a companion is recorded,
  `doctor` says so, so a before/after comparison is not attributed to routing
  alone.

## Headroom (default companion)

[Headroom](https://github.com/chopratejas/headroom), Apache 2.0, is a local
proxy that compresses what the agent reads (tool results, logs, files, search
hits, history) before it reaches the model API, keeps the originals for
retrieval, and wraps supported CLIs with one command. It addresses the lever
Downshift's own native compressor only observes: on Claude Code a PostToolUse
hook cannot replace a tool result, a proxy can.

**How the two fit.** Headroom sits between the harness and the API, so the
Downshift hook still sees the original tool output and the original task
text; routing decisions do not change. Downshift's native provider keeps
recording observe-mode compaction counts. Nothing in either tool needs to
know about the other.

**What Downshift does with it** (all read-only against Headroom):

| Command | Effect |
|---|---|
| `downshift context status` | prints `Companion available: headroom vX detected` when the binary is on PATH and not recorded, or `Companion: headroom (recorded as active)` |
| `downshift context enable` | enables the native provider and, if Headroom is detected, records it as companion (prints the wrap command per harness) |
| `downshift context enable headroom` | records the companion only; refuses with the install command when the binary is absent |
| `downshift context disable headroom` | clears the record; `downshift context disable` clears provider and companion |
| `downshift doctor` | a `companion` row: ok when recorded and present; warn with the exact fix when detected but not recorded, or recorded but missing; silent (verbose: ok) when absent |
| `downshift context providers` / `context doctor` | capability and per-harness table below, plus the privacy note |

**Per harness** (Headroom README compatibility table, read 2026-10-17; the
command is Headroom's, run it yourself):

| Harness | Headroom support | Command |
|---|---|---|
| Claude Code | full | `headroom wrap claude` · undo `headroom unwrap claude` |
| Codex | full | `headroom wrap codex` · undo `headroom unwrap codex` |
| Cursor | manual | `headroom wrap cursor` starts the proxy and prints base URLs for Cursor's settings |
| Antigravity | not listed | any OpenAI-compatible client can target `headroom proxy`; not verified here |
| KiroCrew | not listed | not verified here |

**Install** (Headroom's own instructions): `uv tool install --python 3.13
"headroom-ai[all]"` or `pip install "headroom-ai[all]"`. The npm package is
the SDK only and has no CLI.

**Privacy.** Headroom's anonymous beacon is on by default upstream; set
`HEADROOM_BEACON=off` or `DO_NOT_TRACK=1` to turn it off. Downshift sends
nothing and stores no prompt, with or without a companion.

**The portable `headroom-skill`** (a re-implementation of the techniques as
agent rules plus Python helpers, Apache 2.0) works by adding rules to
`CLAUDE.md` or `AGENTS.md` in every project. That is always-loaded context in
every session, the cost the [audit skill](../../skills/downshift-audit/SKILL.md)
measures. Prefer the binary; if you use the skill, keep it in one project and
measure.

## OmniRoute (optional gateway)

[OmniRoute](https://github.com/diegosouzapw/OmniRoute), MIT, is a self-hosted
OpenAI-compatible gateway: one local endpoint, many providers, quota-aware
fallback, its own compression. It is complementary, as
[when-to-use.md](when-to-use.md) already says of LiteLLM and OpenRouter:
the gateway chooses transport, credentials and provider; Downshift chooses
the subagent's model id and effort at spawn time.

**What works.** OmniRoute exposes provider-prefixed ids. Downshift strips the
first `provider/` segment (the OpenRouter rule) and resolves the rest by exact
id, alias or family, so an Anthropic-family id behind any prefix routes
normally: `cc/claude-opus-4-8` resolves to the frontier tier.

**What does not, by design.** A non-Anthropic or combo id (`glm/glm-5.2`,
`kmc/kimi-k2.6`, `claude/combo/custo-otimizado`) is not in the catalog.
Downshift then has no baseline tier, holds the spawn and leaves the model
unchanged. That is the fail-open rule, not a bug: doubt never spends money.
Both behaviours are pinned by `TestLookupByID_OmniRoute*` in
`internal/catalog/catalog_test.go`.

**Recipe (Claude Code, from OmniRoute's own guide).** Point Claude Code at the
gateway root with `ANTHROPIC_BASE_URL` (no `/v1`) and `ANTHROPIC_AUTH_TOKEN`,
restart the session, install the Downshift hooks as usual. The session model
Claude Code reports is whatever OmniRoute's `/model` picker or
`ANTHROPIC_MODEL` names; if that is not an Anthropic-family id, add a
[`session-models.json`](../reference/session-models.md) whose ids are in the
catalog, or expect `unknown` baselines in `downshift stats`. Not verified on a
live OmniRoute instance as of 2026-10-17; evidence welcome under
`docs/evidence/`.

## claude-code-setup (pattern, not a dependency)

Anthropic's official
[claude-code-setup](https://github.com/anthropics/claude-plugins-official/tree/main/plugins/claude-code-setup)
plugin scans a repository and recommends hooks, skills, MCP servers,
subagents and commands. It is read-only and Claude Code only. Downshift does
not integrate with it; it follows the same contract: `downshift setup
--dry-run` shows the diff, `setup` asks before writing, `setup --undo`
restores the backup, and the [audit skill](../../skills/downshift-audit/SKILL.md)
never writes.

## task-observer (optional misroute collector)

[task-observer](https://github.com/rebelytics/one-skill-to-rule-them-all),
CC BY 4.0, records corrections you make during a session and proposes skill
updates for your approval. It costs tokens at session start rather than
saving them, so it is not shipped here. One use fits Downshift: when you
correct a subagent that failed because its model was too small, the recorded
observation is the raw material for a `misroute:` issue
([CONTRIBUTING.md](../../CONTRIBUTING.md)) or a reviewed `required_tier`
label. Keep the task text out of the issue; `downshift try "<task>"` output
and the expected tier are enough.

## Cost and usage skills (overlap, not companions)

Skills such as `usage-limit-reducer` or `claude-cost-optimizer` read local
transcripts and suggest, among other rules, "use a cheaper model for simple
tasks". That is the rule Downshift applies deterministically at spawn time,
so they overlap rather than complement. Their fixed-context advice (smaller
`CLAUDE.md`, fewer always-loaded tools) is covered by section 3 of the
[audit skill](../../skills/downshift-audit/SKILL.md). Their dollar figures
are list-price estimates from transcripts; on a subscription plan there is no
per-token invoice, the same caveat `downshift stats` prints.

## Related

- [when-to-use.md](when-to-use.md): Downshift versus gateways and SDKs
- [design/context-optimization.md](../design/context-optimization.md): the native compressor, providers and companions
- [reference/harness-matrix.md](../reference/harness-matrix.md): what each harness honors
