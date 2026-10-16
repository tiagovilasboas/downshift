---
name: downshift-audit
description: Read-only audit of a user's Downshift setup and fixed agent context. Use when asked why Downshift does not rewrite models, whether hooks are wired, where subagent or context tokens go, or to review CLAUDE.md/AGENTS.md size.
---

# Downshift audit

Read-only. Never edit settings, hooks or context files without the user's approval. Never read or print task prompts. Never promise a savings percentage: Downshift reports estimated savings, not billed cost.

## 1. Is it running?

1. `downshift doctor --json`: a checklist (`ok`/`warn`/`fail`, each with a `fix`). It covers the binary vs the hook command per harness, hooks wired, both state dirs, the session-model source, the last hook event and whether a rewrite was honored. Report the failed checks first.
2. Hook wiring: find where the user's harness keeps its hooks (`docs/guides/install.md` has one section per harness: Claude Code, Cursor, Codex, Antigravity, KiroCrew; Grok uses config, not a hook) and show only the hooks block. Expect a pre-spawn hook that runs `downshift <harness>`. Optional post-spawn hooks add honor and usage evidence where the harness supports them (`docs/reference/harness-matrix.md`).
3. Session models: Claude Code infers them from the current model when no list exists; other harnesses need a hook list, `downshift models discover` or `~/.downshift/session-models.json` (least to most capable). Missing hooks are fixed by `downshift setup --dry-run` then, with the user's approval, `downshift setup`.

## 2. What did it do?

- `downshift stats --days=7`: decisions and estimated savings. `--export` gives a prompt-free summary.
- `downshift quota status --harness <harness>`: only when quota evidence is in use.
- `downshift doctor` also lists model-override variables that are set (for any harness that declares one).
- Project floor: if `.downshift/policy.json` exists in the project, read it; an invalid file is ignored (fail-open), so validate its tiers and risk names by eye.
- `downshift try "<task>" <harness>`: reproduce one decision.

## 3. Fixed context cost

Measure what loads every session. List the files the user's harness loads automatically (the paths below cover the common ones; skip what does not exist):

```bash
wc -c CLAUDE.md AGENTS.md GEMINI.md .claude/rules/* .cursor/rules/* .kiro/steering/* \
  ~/.claude/CLAUDE.md ~/.codex/AGENTS.md 2>/dev/null | sort -rn
```

Add the skill descriptions (the `description:` line of each `SKILL.md`). Estimate tokens as bytes / 4 and say it is an estimate. For each large always-loaded file, ask whether it applies to every project. Content for one domain belongs in a manual or file-matched rule.

## 4. Report

Table: finding | evidence (command and the line you read) | suggested fix | confidence (observed, inferred, unverified). Top three findings first. Mark anything you could not run as unverified.

Symptom-to-cause hints: [references/interpretation.md](references/interpretation.md).

harness-downshift by Tiago de Carvalho Vilas Boas · https://github.com/tiagovilasboas/downshift
