# When to use Downshift

Downshift is an **optimizer for the cost and efficiency of agent execution**, whose main lever today is a **deterministic model router** that runs **locally** and hooks into **coding harnesses** (Claude Code, Cursor, Codex, and others) at subagent spawn time. It is not a replacement for every LLM integration pattern.

Every harness with an adapter gets every feature, computed in shared layers. **Claude Code is the primary harness**: the most complete and the most verified. Start there if you can choose; the [harness matrix](../reference/harness-matrix.md) says what the others honor today. Harnesses carry one of three status words: Completed (Claude Code), Functional (Codex, Cursor) or Experimental (KiroCrew, Antigravity; policy mode or proof of concept), defined in the [status table](../reference/harness-matrix.md#status-per-harness).

## Use Downshift when

- You run **agentic coding sessions** where subagents inherit the session’s most expensive model.
- You want **rule-based routing** (classifier + catalog policy) with **no LLM in the routing loop**.
- You need **offline, keyless tier selection** for hooks: classify on CPU, pick model from your catalog, rewrite `updatedInput` before the child starts.
- You care about **testability**: `downshift try`, [benchmark/report.md](../../benchmark/report.md), optional shadow classifier (`DOWNSHIFT_SHADOW_WEIGHTS`).
- You want **local telemetry** (`events.jsonl`) and explicit training labels (`--required-tier`), not a hosted analytics product.

## Consider something else when

| Need | Better fit | Why |
|------|------------|-----|
| **Unified HTTP API** to 100+ providers, keys, retries, fallbacks | [LiteLLM](https://github.com/BerriAI/litellm) (or similar gateways) | Downshift does not terminate HTTP chat/completions today; it routes at the harness hook layer. |
| **Hosted model marketplace** and billing in one dashboard | [OpenRouter](https://openrouter.ai/) (or your cloud’s model API) | Downshift does not proxy traffic or add a provider markup layer. |
| **Self-hosted gateway** with many providers and quota fallback | [OmniRoute](https://github.com/diegosouzapw/OmniRoute) | Same split: the gateway picks provider and transport, Downshift picks the subagent's model id. Anthropic-family ids behind a prefix route; others stay unknown and fail open. [companions.md](companions.md). |
| **Compress what the agent reads** (tool output, logs, files) in the request path | [Headroom](https://github.com/chopratejas/headroom), the default companion | Downshift's native compressor only observes on Claude Code (a hook cannot replace a tool result); a proxy can. Downshift detects and records Headroom, never wraps it. |
| **One model for everything** in a simple script | Direct provider SDK | No spawn delegation → little value from subagent routing. |
| **LLM decides which model to use** | Router products that call a model for routing | Downshift deliberately avoids that on the hot path. |

## Downshift + gateways and compressors (complementary)

Many teams use **LiteLLM, OpenRouter or OmniRoute behind the harness** for provider access, and **Downshift in front of subagent spawns** for tier selection. Those layers solve different problems: gateway = transport and credentials; Downshift = **which model id / effort** this spawn should use. A compression proxy such as Headroom is a third layer that neither of the others replaces. Which companions Downshift detects, and what it will never do for them: [companions.md](companions.md).

What each saving claim must prove (same provider smaller model, same model lower effort, cross-provider) and where Downshift adds value beyond native `opusplan` and gateways, with each claim marked shown or unproven: [evidence.md](evidence.md).

## Today vs roadmap

- **Today:** PreToolUse hooks, single Go binary, embedded catalog, user override under the [state dir](config.md) (`~/.downshift`, legacy `~/.harness-downshift`).
- **Not today:** HTTP gateway, multi-tenant hosted router, or automatic training without reviewed labels.

For roadmap and doc tiers, see [ROADMAP.md](../../ROADMAP.md) and [publishing-boundaries.md](../reference/publishing-boundaries.md).
