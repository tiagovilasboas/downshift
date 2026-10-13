<!-- Copyright (c) 2026 Tiago de Carvalho Vilas Boas. SPDX-License-Identifier: Apache-2.0 -->

# N3: stale quota stops holding; `downshift config`

- Today a stale quota snapshot holds every rewrite forever (found on 2026-10-13: a single expired
  statusline snapshot silently disabled Claude Code routing). New rule: a snapshot is `stale` (holds)
  for up to 1 h after `expires_at`; after that it is treated as absent (`unknown`, the default for
  users without quota). `DOWNSHIFT_QUOTA_MODE=required` keeps holding (explicit opt-in).
- `doctor` adds a check: quota snapshot holding rewrites (warn with age and fix); the statusline
  summary appends `· quota stale` while a stale snapshot holds.
- `downshift config [--json]`: every effective setting with its value and source (env, file,
  default): state dir, session-model source per harness, quota mode, scores on/off and routing mode,
  control share, quiet, catalog source, semantic boost, infer session. Read-only.
- Tests at the shared layer for the 1 h boundary, required mode, doctor check, config sources.
- Docs: `docs/quota.md` rule update, `docs/config.md`.
