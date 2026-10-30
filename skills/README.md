# Skills shipped with Downshift

Four skills, four jobs, no overlap. They are plain `SKILL.md` files any
harness can read; the Claude Code plugin ships them as `/downshift:<name>`.

| Skill | Job | Writes? | When to stop using it |
|---|---|---|---|
| `downshift-setup` | install, repair and first validation of the hooks; dry-run before any write, backups and `--undo` | yes, after a yes | once `downshift doctor` is clean; a hand-copied install can delete it |
| `downshift-audit` | independent, read-only inspection: is it wired, what did it do, what loads every session | never | keep; it is the second opinion |
| `harness-engineer` | six steps: restate, split by story points, parallelize, keep small work local, leave mechanical spawns unpinned, check with `session --why` and `report` | never | keep while you delegate work |
| `cost-reducer` | the one entry point for "we spend too much": fixed diagnosis order, biggest lever first, one change at a time | only after a yes | keep; it defers to setup when nothing is wired |

`downshift-effort` is a fifth, advisory skill: it tells the orchestrator how much
reasoning and verification a task's stakes deserve (mechanical, ordinary, high) and
never sets an effort value itself. Setup installs it next to `downshift-orchestrator`.
Inspired by an MIT-licensed skill, github.com/mmc7676/skills.

## What they cost

A skill is not free: its `description` is loaded into every session, and its
body when it triggers. Measured on 2026-10-17 as bytes / 4 (an estimate, not a
tokenizer):

| File | Description, always loaded | Body, loaded on use |
|---|---|---|
| `cost-reducer/SKILL.md` | ~86 tokens | ~750 tokens, plus the one reference file it opens (~400 each) |
| `downshift-audit/SKILL.md` | ~70 tokens | ~890 tokens |
| `downshift-setup/SKILL.md` | ~66 tokens | ~730 tokens |
| `harness-engineer/SKILL.md` (2026-10-21) | ~50 tokens | ~170 tokens |
| `agents/mechanical-worker.md` | ~55 tokens | ~270 tokens |
| `commands/status.md` | ~16 tokens | ~90 tokens |

About 340 tokens per session for the whole set when installed through the
plugin. A skill does not save anything by recommending savings; it pays off
only when its advice is applied. Descriptions therefore trigger only when the
user names Downshift or asks about its hooks and cost, not on any mention of
`CLAUDE.md` or billing.

## Delegation is not free either

`agents/mechanical-worker.md` is an example of a bounded, unpinned subagent
that Downshift can right-size. It is not a recommendation to delegate every
small task: a spawn pays for its own context, the brief, the return and the
parent's coordination, and a one-line change is usually cheaper done by the
main agent. Delegate when the work is self-contained and its output is short;
then let the router pick the tier. See [docs/guides/subagents.md](../docs/guides/subagents.md).

Downshift by Tiago Vilas Boas · https://github.com/tiagovilasboas/downshift
