# When to use Downshift

Downshift is a **deterministic model router** that runs **locally** and hooks into **coding harnesses** (Claude Code, Cursor, Codex, and others) at subagent spawn time. It is not a replacement for every LLM integration pattern.

## Use Downshift when

- You run **agentic coding sessions** where subagents inherit the session’s most expensive model.
- You want **rule-based routing** (classifier + catalog policy) with **no LLM in the routing loop**.
- You need **offline, keyless tier selection** for hooks: classify on CPU, pick model from your catalog, rewrite `updatedInput` before the child starts.
- You care about **testability**: `downshift try`, [benchmark/REPORT.md](../benchmark/REPORT.md), optional shadow classifier (`DOWNSHIFT_SHADOW_WEIGHTS`).
- You want **local telemetry** (`events.jsonl`) and explicit training labels (`--required-tier`), not a hosted analytics product.

## Consider something else when

| Need | Better fit | Why |
|------|------------|-----|
| **Unified HTTP API** to 100+ providers, keys, retries, fallbacks | [LiteLLM](https://github.com/BerriAI/litellm) (or similar gateways) | Downshift does not terminate HTTP chat/completions today; it routes at the harness hook layer. |
| **Hosted model marketplace** and billing in one dashboard | [OpenRouter](https://openrouter.ai/) (or your cloud’s model API) | Downshift does not proxy traffic or add a provider markup layer. |
| **One model for everything** in a simple script | Direct provider SDK | No spawn delegation → little value from subagent routing. |
| **LLM decides which model to use** | Router products that call a model for routing | Downshift deliberately avoids that on the hot path. |

## Downshift + gateways (complementary)

Many teams use **LiteLLM or OpenRouter behind the harness** for provider access, and **Downshift in front of subagent spawns** for tier selection. Those layers solve different problems: gateway = transport and credentials; Downshift = **which model id / effort** this spawn should use.

## Today vs roadmap

- **Today:** PreToolUse hooks, single Go binary, embedded catalog, user override under the [state dir](config.md) (`~/.downshift`, legacy `~/.harness-downshift`).
- **Not today:** HTTP gateway, multi-tenant hosted router, or automatic training without reviewed labels.

For roadmap and doc tiers, see [ROADMAP.md](../ROADMAP.md) and [publishing-boundaries.md](publishing-boundaries.md).
