<!-- Copyright (c) 2026 Tiago de Carvalho Vilas Boas. SPDX-License-Identifier: Apache-2.0 -->

# UX review 2026-10-13: make Downshift easy to start and its value obvious

Evidence: real outputs of `help`, `try`, `doctor`, `stats`, the dashboard and the
README quickstart on the maintainer Mac.

| ID | Pri | Problem (evidence) | Fix |
|----|-----|--------------------|-----|
| X1 | P0 | Setup is manual JSON editing; README shows 1 of the 5 hooks, so cost, child status and the statusline never get wired; matcher says `Task` while the tool is now `Agent` | `downshift setup [harness]`: detect harnesses, merge every hook and the statusline into the harness settings with backup, diff, `--dry-run`, idempotent, uses the absolute binary path; `--undo` restores the backup |
| X2 | P0 | `doctor` dumps key:value lines and a 27-model wall; it never checks the hooks, so a hook pointing at another binary went unnoticed | `doctor` as a checklist with ✓/✗ and one fix command per ✗: binary on PATH = hook command, hooks wired per harness, last hook event age, session list resolvable, rewrite honored seen, scores cache; details behind `--verbose` |
| X3 | P0 | `stats` is 40 lines; the headline (net $) is buried; "Normalised savings 0.0%" contradicts "Real saved $33.72" | First block: `Last 24h · 67 subagents · net saved $15.51 (saved $33.72, cost more $18.21) · 29 risky moves held`; normalised units and honor/quota/usage move under `--detail` |
| X4 | P1 | First `try` says "Recommend: haiku" then "Rewrite: no (keep current model)" because no current model was given, so it looks broken | Without a current model, assume the harness default (catalog frontier for Claude Code) and say so: `from opus → haiku (~98% cheaper)` plus one "why" line; jargon (Intent/Verdict/Confident) under `--detail` |
| X5 | P1 | README first screen is about mechanism and maintainer eval numbers (shadow memory, 38/40 LOO) | Lead with the benefit, one proof line, 2-command start (`install`, `setup`), what you will see (statusline, child line); move eval prose to docs |
| X6 | P2 | Dashboard is a flat list with `unknown` rows and no cost | Session tree with cost and redo (U3/U4) |
| X7 | P2 | 20+ `DOWNSHIFT_*` env vars, no single view | `downshift config` prints the effective settings and where each comes from |
