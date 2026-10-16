<!-- Copyright (c) 2026 Tiago de Carvalho Vilas Boas. SPDX-License-Identifier: Apache-2.0 -->

# N4: README first screen and quickstart

Lead with the benefit and what the user will see; keep it honest (no unmeasured savings claims).
- First screen: one-sentence value, three bullets (cheaper subagents on easy tasks, guardrails so
  doubt never spends money, see it live in the statusline and `/downshift`), a 2-command start:
  `curl …/install.sh | sh` then `downshift setup` (show `--dry-run` first), then "open a new session".
- Move maintainer eval numbers (shadow memory, 38/40 LOO) out of the first screen to
  docs/reference/complexity-weights.md or benchmark/report.md links.
- Quickstart: replace manual JSON editing with `setup`; keep a collapsed "manual setup" pointer to
  docs/guides/install.md. Matcher is `Agent|Task`. Mention `doctor`, `stats`, `session`, `try`.
- docs/guides/install.md: the same order at the top. Keep attribution.
