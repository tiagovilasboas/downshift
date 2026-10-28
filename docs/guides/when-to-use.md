# When to use Downshift

Downshift gives your coding agent the right model and less noise, with no extra AI call. It runs locally, hooks into your coding harness, and rests on three pillars, in this order:

1. **Model delegation per subagent.** The orchestrator chooses model and effort for each spawn; Downshift applies the choice within your declared policy, and when unsure your model stays. Implemented; honored on Claude Code (Completed), inferred on Codex and Cursor (Functional).
2. **The Compactor** (`downshift compact`). Shorter output from allowlisted commands, exit code kept, failures left raw. Implemented, on by default. Automatic rewrite is Claude Code only; elsewhere the agent runs `downshift compact -- <command>`.
3. **DCI.** Opt-in; the agent reads only the code a change touches. Implemented (v0.4); bytes and latency measured, answer quality not measured.

Next: [install](install.md), [how it works](../design/architecture.md), [evidence](evidence.md). Numbers and their limits are in [evidence.md](evidence.md): the routing trial is up to about 12% on mechanical tasks (one operator, three runs per cell, one controlled run), and compaction is bytes on a synthetic corpus (82.3% on the 26 of 248 files compacted, 23.7% overall).

Every harness with an adapter gets every feature, computed in shared layers. **Claude Code is the primary harness**: the most complete and the most verified. Start there if you can choose; the [harness matrix](../reference/harness-matrix.md) says what the others honor today. Harnesses carry one of four status words: Completed (Claude Code), Functional (Codex, Cursor), Beta (OpenCode; first-class integration in development) or Experimental (KiroCrew, Antigravity; policy mode or proof of concept), defined in the [status table](../reference/harness-matrix.md#status-per-harness).

## Use Downshift when

- You run **agentic coding sessions** where subagents inherit the session’s most expensive model.
- You want **declared policy** (profiles, rules, floors) bounding the orchestrator's model choices, with **no LLM call in Downshift's own code**.
- You need **offline, keyless tier selection** for hooks: classify on CPU, pick model from your catalog, rewrite `updatedInput` before the child starts.
- You care about **testability**: `downshift try`, [benchmark/report.md](../../benchmark/report.md).
- You want **local telemetry** (`events.jsonl`) and explicit training labels (`--required-tier`), not a hosted analytics product.

## Consider something else when

| Need | Better fit | Why |
|------|------------|-----|
| **Unified HTTP API** to 100+ providers, keys, retries, fallbacks | A model gateway | Downshift does not terminate HTTP chat/completions today; it routes at the harness hook layer. |
| **Hosted model marketplace** and billing in one dashboard | A hosted model API from your provider | Downshift does not proxy traffic or add a provider markup layer. |
| **Compress what the agent reads** (tool output, logs, files) in the request path | A compression proxy (a companion, detected and recorded) | The Compactor shortens the output of allowlisted commands (it wraps every `git` subcommand and the listed test and build families), but a hook cannot replace an arbitrary tool result; a proxy can. Downshift detects and records a companion, never wraps it. |
| **One model for everything** in a simple script | Direct provider SDK | No spawn delegation → little value from subagent routing. |
| **A separate model call that picks the model** | Router products that call a model for routing | Downshift adds no such call. The orchestrator you already run chooses the model; Downshift applies it, bounds it with your policy, and measures. |

## Downshift + gateways and compressors (complementary)

Many teams use **a model gateway behind the harness** for provider access, and **Downshift in front of subagent spawns** for tier selection. Those layers solve different problems: gateway = transport and credentials; Downshift = **which model id / effort** this spawn should use. A compression proxy is a third layer that neither of the others replaces. Which companions Downshift detects, and what it will never do for them: [companions.md](companions.md).

What each saving claim must prove (same provider smaller model, same model lower effort, cross-provider) and where Downshift adds value beyond native `opusplan` and gateways, with each claim marked shown or unproven: [evidence.md](evidence.md).

## Today vs roadmap

- **Today:** PreToolUse hooks, single Go binary, embedded catalog, user override under the [state dir](config.md) (`~/.downshift`, legacy `~/.harness-downshift`).
- **Not today:** HTTP gateway, multi-tenant hosted router, or automatic training without reviewed labels.

For roadmap and doc tiers, see [ROADMAP.md](../../ROADMAP.md) and [publishing-boundaries.md](../reference/publishing-boundaries.md).
