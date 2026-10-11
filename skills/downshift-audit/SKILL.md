---
name: downshift-audit
description: Read-only audit of a user's Downshift setup and fixed agent context. Use when asked why Downshift does not rewrite models, whether hooks are wired, where subagent or context tokens go, or to review CLAUDE.md/AGENTS.md size.
---

# Downshift audit

Read-only. Never edit settings, hooks or context files without the user's approval. Never read or print task prompts. Never promise a savings percentage: Downshift reports estimated savings, not billed cost.

## 1. Is it running?

1. `downshift doctor`. Note `state_dir`, `catalog`, and whether `events` exists. "not created yet" means no hook has fired.
2. Hook wiring: show only the `hooks` block of `~/.claude/settings.json` (or the harness equivalent). Expect `PreToolUse` with matcher `Task` running `downshift claude-code`. Optional: `PostToolUse` and `SubagentStop` for honor and usage.
3. Session models: `~/.downshift/session-models.json` (or `DOWNSHIFT_SESSION_MODELS`) must list the user's models least to most capable. Without it the hook never rewrites.

## 2. What did it do?

- `downshift stats --days=7`: decisions and estimated savings. `--export` gives a prompt-free summary.
- `downshift quota status --harness <harness>`: only when quota evidence is in use.
- `downshift try "<task>" <harness>`: reproduce one decision.

## 3. Fixed context cost

Measure what loads every session:

```bash
wc -c CLAUDE.md AGENTS.md .claude/rules/* ~/.claude/CLAUDE.md 2>/dev/null | sort -rn
```

Add the skill descriptions (the `description:` line of each `SKILL.md`). Estimate tokens as bytes / 4 and say it is an estimate. For each large always-loaded file, ask whether it applies to every project. Content for one domain belongs in a manual or file-matched rule.

## 4. Report

Table: finding | evidence (command and the line you read) | suggested fix | confidence (observed, inferred, unverified). Top three findings first. Mark anything you could not run as unverified.

Symptom-to-cause hints: [references/interpretation.md](references/interpretation.md).

harness-downshift by Tiago de Carvalho Vilas Boas · https://github.com/tiagovilasboas/downshift
