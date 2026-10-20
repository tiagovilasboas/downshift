# Agents (layer 4, delegation)

One example subagent ships with the plugin: [`mechanical-worker`](../../agents/mechanical-worker.md),
scoped as `downshift:mechanical-worker` when installed. It is not the point of
Downshift; it shows the one rule that decides whether routing can act at all:

**A subagent is routable only when its model is not pinned.** With no `model`
in the frontmatter (and no `CLAUDE_CODE_SUBAGENT_MODEL` in the environment), a
Claude Code subagent inherits the session model, and the `PreToolUse` hook may
rewrite that spawn to a right-sized model from the task text. A frontmatter
`model: opus` (or `sonnet`, `haiku`, `fable`, a full id) is an explicit choice:
Downshift sees it as the requested model and never overrides an explicit pick
that it cannot read as inherited. `model: inherit` behaves like no model.

So, for agents you write yourself:

| You want | Frontmatter |
|---|---|
| Downshift to pick the tier per task | omit `model` (or `model: inherit`) |
| One fixed model, always | `model: <alias or id>`; routing leaves it alone |

**Delegation has a cost of its own.** A spawn pays for the child's context, the
brief, the return and the parent's coordination; Downshift records that cost
as tokens per child on Claude Code (`downshift stats`, `downshift session --tree`) but
does not decide whether to delegate. A one-line edit is usually cheaper in the
main agent even on a frontier model. The example agent shows what a routable
delegation looks like, not that every trivial task should become one.

The classifier reads the Task prompt, not the agent file. A bounded, mechanical
brief routes small; a vague or risky one keeps the session model. `downshift try
"<the brief>"` shows the decision before any spawn.

Other harnesses have no agent files in this format; the same rule applies to
whatever field pins a child's model there ([harness matrix](../reference/harness-matrix.md)).

Downshift by Tiago Vilas Boas · https://github.com/tiagovilasboas/downshift
