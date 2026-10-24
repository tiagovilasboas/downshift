<!-- Copyright (c) 2026 Tiago Vilas Boas. SPDX-License-Identifier: Apache-2.0 -->

# Design note: `subagent_type` is not a routing signal (FinOps P1-5)

Downshift by Tiago Vilas Boas
https://github.com/tiagovilasboas/downshift

Status: investigation, 2026-10-19. No routing behaviour changed by this note.

## Question

`agents/mechanical-worker.md` declares no `model:`. Does the hook treat the
Task call's `subagent_type` (for example `mechanical-worker`) as information?

## What the code does today

- The tier comes from the task text only (`TaskText`: `prompt`, `description`).
  `subagent_type` is never read by the classifier or by `core.Route`.
- The adapter reads it for two things: the baseline model
  (`inheritsSessionModel`, `internal/adapters/claudecode/sessionmodel.go`) and a
  storable telemetry label (`telemetry.AgentTypeLabel`; custom names collapse to
  `custom`).
- Only `""`, `general-purpose`, `claude`, `Explore` and `Plan` inherit the parent
  session model. Every other type, including a custom one with no `model:`
  line, gets an unknown baseline: the parent model "would be a guess" because a
  type may pin its own model in its definition.

## Observed (isolated HOME, Opus parent in the transcript, same trivial rename prompt)

| `subagent_type` | Rewrite | Baseline | Message |
|---|---|---|---|
| `general-purpose` | `model: haiku` | parent (Opus) | "downshift to claude-haiku-4-5 (~75% cheaper)" |
| `mechanical-worker` | `model: haiku` | unknown | "use claude-haiku-4-5 (small tier)", no saving claim |

So routing works for the custom type, because the text is classified. What is
lost is the baseline: the spawn is not counted as a measurable saving, and
`stats` reports an unknown-baseline rate for it (`stats --detail`, "Baseline by
agent type"). That is conservative and correct today, not a bug.

## The real risk is the other direction

A per-call `model` wins over the agent definition in Claude Code. If a custom
type pins `model: opus` (or `haiku`) in its frontmatter, the hook, which cannot
see the definition, rewrites over that pin: a confident downshift removes a
deliberate quality choice, and a confident upshift spends money the author did
not ask for. The guard that exists is `explicit_only` models and the held
unconfident upshift; neither knows about frontmatter.

## Options

1. **Use the type name as a signal.** Rejected. Names are free text chosen by
   the user, not a stable vocabulary; `mechanical-worker` says nothing the task
   text does not, and a name such as `reviewer` would wrongly pull work up or
   down. It would add a hidden, hard-to-test input to a deterministic router.
2. **Read the agent definition's `model:` (smallest safe change).** For a type
   name matching `^[A-Za-z0-9._-]{1,64}$` (no separators, no plugin `name:agent`
   scope), read `<cwd>/.claude/agents/<type>.md` and `~/.claude/agents/<type>.md`
   with a size cap, parse only the frontmatter `model:` line, and:
   - no `model:` or `inherit` -> treat like an inheriting type (baseline = parent
     model, savings measurable);
   - a pinned model -> baseline = that model, and treat it as an explicit choice
     (`ShouldPreserveExplicitModel` path: never upshift, downshift only if the
     owner opts in);
   - file missing or unreadable -> today's behaviour (unknown baseline).
   Cost: up to two small reads per spawn (about 20-50 µs on warm cache, not
   measured). Plugin agents (`downshift:mechanical-worker`) live under the
   plugin cache path and need the plugin manifest to resolve; out of scope for
   a first step.
3. **Do nothing.** Acceptable for savings accuracy; leaves the override-a-pin
   risk, which needs data to size.

## Recommendation

Do not change routing yet. Ship option 2 as **observation only** first: record
a prompt-free field on the event (`agent_def_model`: `none|inherit|pinned`) so
`stats --detail` can say how many custom-type spawns carry a pin. Only that
measurement can justify changing behaviour, and the fine-tuning rule requires a
measured before/after. There is no checked-in dataset with agent definitions, so
none of the above has a measured effect size yet.

Not measured: how often real users' custom agents pin a model; the file-read
cost; Claude Code versions where frontmatter `model:` precedence differs.

## Addendum 2, 2026-10-19: option 2 shipped, and it routes

The owner reported implausibly low savings on a day with 61 subagent decisions. Option 2 was
built as a routing change, not observation only, because the cost of waiting was measured:
`claude-code-guide` (runs on Haiku by its definition) and custom or plugin agents were held or
left without a baseline, so their spawns could not count as a saving. `lookupAgentDefinition`
(`internal/adapters/claudecode/agentdef.go`) reads only the frontmatter `model:` and `name:`
lines, with a 64 KiB cap and a type-name pattern that cannot leave an agents directory, and
resolves plugin agents through `installed_plugins.json`, which the first step left out of scope.
The pin is a core fact (`Baseline.Pinned`, reason `AGENT_PINS_MODEL`) and `PlanForSession`
holds it like an `explicit_only` model, so no adapter owns that policy. The type name is still
not a routing signal (option 1 stays rejected): `downshift:mechanical-worker` routes small
because its brief classifies small, with the risk floors intact. Still not measured: how often
real custom agents pin a model, and the file-read cost.

## Addendum, 2026-10-19: the measured "custom" share was not custom agents

`stats --detail` on the operator's log showed 72 of 72 `custom` decisions with
an unknown baseline. None of them was a Claude Code custom agent: each sat
within 3 s of a cursor decision, and every one with a tool-call id shared it
with that decision. Cursor runs the Claude Code hooks when its third-party
configs are on, and its Task input (`model: "inherit"`, lowercase types such as
`explore`) was logged a second time under claude-code. The shared runners now
leave such a copy to the cursor hook (`PayloadMarkers`, see
[harness-matrix.md](../reference/harness-matrix.md)). The same log's Claude
Code transcripts named only `general-purpose`, `Explore` and
`claude-code-guide`, so there is still no measurement of real custom types, and
the recommendation above stands.

## Addendum 3, 2026-10-23: the built-in type is a hint, not a name match

Option 1 stays rejected for free-text names. Two Claude Code built-ins have a fixed job
(`Explore` read-only search, `Plan` design), and the owner's log showed 4 of 7 `Explore`
spawns ending UNKNOWN or held and 46 `general-purpose` decisions CLASSIFIER_UNCONFIDENT.
`HarnessCapabilities.AgentTypeHints` (data) plus `core.RouteAgent` now let the type fill doubt,
never override evidence: only when the text read is unconfident, with no risk signal and no
COMPLEX read, with the current model known and the hint capped at its tier. A confident
classifier read wins over the type in both directions (owner can flip this in
`agentTypeHintTier`). Reason code `AGENT_TYPE_HINT`. Cursor and Codex pass their field, with no
hint names until evidence.
