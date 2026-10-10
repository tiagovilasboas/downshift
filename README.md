<div align="center">

<picture>
  <source media="(prefers-color-scheme: dark)" srcset="docs/brand/downshift-logo-dark.svg">
  <img src="docs/brand/downshift-logo-light.svg" alt="Downshift" width="320">
</picture>

**Deterministic model routing for coding-agent subagents. No LLM in the routing loop.**

[![Build](https://github.com/tiagovilasboas/downshift/actions/workflows/ci.yml/badge.svg)](https://github.com/tiagovilasboas/downshift/actions/workflows/ci.yml)
[![License: Apache-2.0](https://img.shields.io/badge/license-Apache--2.0-blue.svg)](LICENSE)
[![Go 1.27](https://img.shields.io/badge/go-1.27-00ADD8.svg)](https://go.dev)
[![Release](https://img.shields.io/github/v/release/tiagovilasboas/downshift?include_prereleases&label=release)](https://github.com/tiagovilasboas/downshift/releases)
[![Status: beta](https://img.shields.io/badge/status-beta-yellow)](#status)

[Install](docs/install.md) · [Docs](docs/README.md) · [Harness support](#harness-support) · [Contributing](#contributing)

<!-- GitHub themed-picture uses the first dark source and ignores max-width, so a mobile source here is stretched to the column width. -->
<picture>
  <source media="(prefers-color-scheme: dark)" srcset="docs/img/hook-flow-dark.svg">
  <img src="docs/img/hook-flow.svg" alt="Hook flow: the harness sends a PreToolUse event, Downshift classifies the task, applies policy against the session models, and returns updatedInput with the chosen model, or allows the spawn unchanged." width="720">
</picture>

</div>

> **Beta.** Whether a rewrite is applied depends on the harness and plan, not only on Downshift. Read [plan compatibility](docs/install.md#plan-compatibility--read-before-installing) before installing, and see [Harness support](#harness-support) for what has been verified.

## What it is

Downshift is a single Go binary that runs as a **hook** in your coding harness. When the harness is about to spawn a **subagent** (a child task), Downshift scores the task text, picks a tier (small / mid / frontier) and a reasoning effort, and rewrites **only that subagent's model** to a right-sized one from your session's available models. The parent session model never changes. No network calls, no API keys.

With no tier memory on disk, the same input yields the same decision. Reviewed outcomes can be stored by complexity class and dominant feature, but that memory is **shadow**: it records the tier it would pick and does not change the spawn. Promotion needs an independent outcome suite and a measured cost per completed task. Neither exists yet. On the maintainer suite, class-scoped shared memory matches the classifier (38/40) offline; per-task memory is an upper bound (40/40), not production. Numbers: [benchmark/REPORT.md](benchmark/REPORT.md). Weights: [docs/complexity-weights.md](docs/complexity-weights.md).

**Example:** Your session runs Claude Sonnet 5.5. You spawn 3 subagents — Downshift may route them to Haiku, Sonnet, and Opus respectively, based on task complexity. Billing and token usage happen at the subagent tier, not the session.

**What it is not:** an HTTP gateway or proxy (that is LiteLLM's job), a hosted model marketplace (OpenRouter), or an LLM-based classifier (Downshift uses deterministic signals + optional local MiniLM semantic scoring). It only acts on subagent spawns inside harnesses that expose a pre-tool hook. Comparison: [docs/when-to-use.md](docs/when-to-use.md).

## How it works

**Session model stays fixed. Subagent models get routed.**

```
Parent (e.g. Claude Sonnet 5.5) spawns 3 tasks:

  Task 1: "rename a variable"       → trivial    → Haiku (¢ cheaper)
  Task 2: "refactor a module"       → normal     → Sonnet (same tier)
  Task 3: "design a new algorithm"  → complex    → Opus ($ more capable)

Billing and token usage happen at the subagent tier, not the session.
```

**The routing loop:**

1. **Intercept.** The harness fires a `PreToolUse` hook when a subagent is about to start. Downshift reads the task text in memory only; prompts are never stored.
2. **Classify.** Deterministic signals (`internal/core`) + optional local MiniLM semantic scoring map the task to TRIVIAL, SIMPLE, MEDIUM, or COMPLEX. Weights and the tier map are in [docs/complexity-weights.md](docs/complexity-weights.md).
3. **Choose.** Policy picks small, mid, or frontier. The target must come from the session's model list, ordered least to most capable ([session-models.md](docs/session-models.md)).
4. **Remember, in shadow.** If `adapt-memory.json` has a reviewed hit or miss for that class and feature, Downshift records the tier it would use. The spawn still uses the classifier tier.
5. **Rewrite or stay out.** Downshift returns the new model in `updatedInput`. If anything is unknown or fails, it does nothing and the spawn runs unchanged (fail-open).

Internals: [docs/architecture.md](docs/architecture.md). The full runtime routing diagram is [here](docs/brand/downshift-routing-runtime-light.svg) ([dark](docs/brand/downshift-routing-runtime-dark.svg)).

<picture>
  <source media="(prefers-color-scheme: dark)" srcset="docs/img/architecture-dark.svg">
  <img src="docs/img/architecture.svg" alt="Architecture: harness adapters call a harness-agnostic core, which uses a model catalog and the session model list to produce a routing decision." width="860">
</picture>

## Quickstart (Claude Code)

**1. Install** (macOS / Linux; see [install.md](docs/install.md) for `go install` and Windows):

```bash
curl -fsSL https://raw.githubusercontent.com/tiagovilasboas/downshift/main/install.sh | sh
```

**2. Tell Downshift which models your session can use.** Without this the hook never rewrites anything. List the model ids from your picker, **least to most capable**, in `~/.downshift/session-models.json` (or point `DOWNSHIFT_SESSION_MODELS` at a file):

```json
{
  "claude-code": ["claude-haiku-5-5", "claude-sonnet-5-5", "claude-opus-5-5"]
}
```

Use the ids your account really offers; the file is yours to maintain. Details and per-session lists: [session-models.md](docs/session-models.md).

Claude Code is not limited to three models. The example stops at Opus on purpose: upshift writes the **last** id in this list, so an expensive frontier model stays out until you opt in. Append `claude-fable-5-1` (the hook writes the family name `fable`) only if you want that upshift.

Or let Downshift ask the harness: `downshift models discover` reads the models your account can use (Codex, Kiro, Grok, Cursor, and the Anthropic API with a key) and writes a ranked cache. Precedence is hook allowlist, configured Cursor native export, discovered cache, then operator file. Antigravity's native CLI can list parent-picker models, but their mapping to routing aliases and quota groups remains unresolved. See [session-discovery.md](docs/session-discovery.md) and the [remaining tasks](docs/gap-tasks.md).

**3. Add the hook** to `~/.claude/settings.json`:

```json
{
  "hooks": {
    "PreToolUse": [
      { "matcher": "Task", "hooks": [{ "type": "command", "command": "downshift claude-code" }] }
    ]
  }
}
```

Claude Code's `Task` schema accepts only family names (`haiku`, `sonnet`, `opus`, `fable`) in `model`. Downshift writes the catalog's family name for you and leaves the spawn alone when a model has none.

**4. Verify.**

```bash
downshift doctor                                          # version, state dir, catalog source
downshift try "rename the userId variable" claude-code    # classification and recommended model
```

Then spawn a trivial subagent in Claude Code; stderr shows a line like `downshift: TRIVIAL task → downshift to …`. That proves the hook ran. To confirm the harness applied the rewrite, follow [Verify the rewrite was honored](docs/install.md#quickstart--zero-to-working-hook-in-2-minutes). Other harnesses (Cursor, Codex, Antigravity, KiroCrew, Grok) are in [install.md](docs/install.md).

## Harness support

| Harness | Subagent tool | Mechanism | Rewrite honored (evidence) |
|---|---|---|---|
| **Claude Code** | `Task` / `Agent`, paid plans | `PreToolUse` → `updatedInput.model` | Yes, observed 2026-10-06 ([write-up](docs/evidence/claude-code-rewrite-honored-2026-10-06.md)) |
| **Codex** | `spawn_agent` (`multi_agent_v2`) | `PreToolUse` → model + `reasoning_effort` | Inferred, not observed, 2026-10-02 ([write-up](docs/evidence/codex-rewrite-honored-2026-10-02.md)) |
| **Cursor** | `Task` | `preToolUse` → `updated_input.model` | Unconfirmed; discarded on Free and legacy Pro plans |
| **Antigravity** | `invoke_subagent` | `PreToolUse` overwrite | Observed 2026-10-09, desktop 2.21.1: [native child evidence](docs/evidence/antigravity-executor-ack.md); quota mapping still unresolved |
| **KiroCrew** | `spawn_run` / `spawn_sub_agents` | Native `preToolUse` policy (exit 0/2); no `updated_input` | Block and retry observed 2026-10-10 on Kiro CLI 2.29; not an in-place rewrite ([write-up](docs/evidence/kirocrew-native-hook-2026-10-10.md)) |
| **Grok CLI** | `spawn_subagent` | Config in `config.toml`, not a hook | No hook rewrite |

Adapters ship for all of the above; the table reports evidence, not just code. Plans, caveats and revalidation rules: [harness-matrix.md](docs/harness-matrix.md).

## Context optimization & native compression

Downshift includes a built-in, zero-dependency **Native Context Compressor** (`downshift context compress`) and event-driven **Context Sensor** to govern context token volume without altering critical failure traces.

- **Enable persists a flag, not a prompt rewrite.** `downshift context enable` saves the native provider as active, plus the exit-code contract. Claude Code PostToolUse then records `CompressExit` in observe mode, using the tool's real exit code. The hook cannot replace tool output, so the model still sees the original result. Prompts are not compressed. Safe mode on the CLI still requires `--exit 0`.
- **Zero external dependencies:** Pure in-process Go engine; zero Python, Rust, Node or LLM runtime requirements.
- **Fail-open & error preservation:** Safe squelch on verbose successes (`go test`, `git status`, logs); never touches errors, stack traces, or failing suites.
- **Bytes, not a token claim.** `downshift context benchmark` prints the four routing scenarios with `not measured` for cost and accuracy, then a measured byte table from checked-in fixtures (original bytes, reduced bytes, byte savings). The unit is bytes. The table is not tokens and not dollars.
- **Management CLI:**
  ```bash
  downshift context status      # Show optimization status & active provider
  downshift context providers   # List supported optimization providers
  downshift context doctor      # Inspect local compressor & harnesses
  downshift context enable      # Enable built-in native compressor (or specify provider)
  downshift context disable     # Revert configurations back to raw state
  downshift context metrics     # Sensor counts; token fields stay unavailable
  downshift context compress    # Compact stdin only with --exit 0 in safe mode
  downshift context benchmark   # Measured fixture bytes; routing cost stays not measured
  ```

See [the context optimization guide](docs/context-optimization.md) for architecture, harness contracts, and failure handling policies.

## Cost and metrics

- **Estimated, not billed.** Routing-only savings use normalized units. With native tokens, `downshift stats --days=7` also calculates catalog-priced costs and a same-token baseline comparison. A [dated audit](docs/evidence/billing-gap-audit.md) matched five linked Claude Code records to native models/tokens, including both positive-saving records. These estimates are not an invoice; the controlled period comparison remains open.
- **Real usage on Claude Code (optional).** Add a `SubagentStop` hook running `downshift claude-code-subagent-stop` to record tokens and cost per subagent from its own transcript. Setup and limits: [session-models.md](docs/session-models.md#claude-code-real-usage).
- **Local only.** Events go to a local `events.jsonl` in the state directory; prompts are not stored. Export format: [stats-export.md](docs/stats-export.md).
- **Classifier numbers.** Published tier accuracy, model pass rates, and the 2026-10-10 adapt comparison (per-task upper bound versus shared-shape memory) are in [benchmark/REPORT.md](benchmark/REPORT.md). Tune the classifier with `downshift try` and [docs/contrib/classifier.md](docs/contrib/classifier.md). Weights: [docs/complexity-weights.md](docs/complexity-weights.md).

## Documentation

[INSTALL](docs/install.md) · [CONFIG](docs/config.md) · [session models](docs/session-models.md) · [session discovery](docs/session-discovery.md) · [ARCHITECTURE](docs/architecture.md) · [complexity weights](docs/complexity-weights.md) · [HARNESS-MATRIX](docs/harness-matrix.md) · [WHEN-TO-USE](docs/when-to-use.md) · [examples](examples/README.md) · [full index](docs/README.md) · [Português](docs/pt/README.md)

Project: [ROADMAP](ROADMAP.md) · [GOVERNANCE](GOVERNANCE.md) · [beta exit criteria](docs/beta-exit.md) · [remaining tasks](docs/gap-tasks.md)

**For AI agents:** start with [docs/install.md](docs/install.md), [AGENTS.md](AGENTS.md) and [llms.txt](llms.txt).

## Contributing

The most valuable contribution is a **misrouted prompt**: open an issue with the `downshift try` output and the tier you expected. Evidence that a rewrite was (or was not) honored on your harness and plan is just as welcome. See [CONTRIBUTING.md](CONTRIBUTING.md).

## Status

**Beta.** The adapters ship, but the limiting factor is often harness or plan support rather than the router. Exit criteria are tracked in [docs/beta-exit.md](docs/beta-exit.md).

## License

Apache License 2.0. See [LICENSE](LICENSE), [NOTICE](NOTICE) and [docs/relicense.md](docs/relicense.md).

Downshift by Tiago de Carvalho Vilas Boas · https://github.com/tiagovilasboas/downshift

Conceptual backbone: [Harness engineering (Fowler)](https://martinfowler.com/articles/harness-engineering.html).
