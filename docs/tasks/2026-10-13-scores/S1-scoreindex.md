<!-- Copyright (c) 2026 Tiago de Carvalho Vilas Boas. SPDX-License-Identifier: Apache-2.0 -->

# S1: public score sources, id map, cache, CLI

Read the research report at
`/private/tmp/claude-501/-Users-tiago-boas-Github-harness-downshift/44afeb9c-62f4-4559-9ba9-0a67f44a9976/scratchpad/public-score-sources.md`
(if it is missing, stop and report). Use only sources it marks VERIFIED, free and
anonymous, with terms that allow automated download and local caching; record
`source`, `url`, `license` per row and put attribution in the docs.

Build `internal/scoreindex`:
- `Source` interface (`Name`, `Fetch(ctx, *http.Client) ([]Row, error)`); try
  sources in order, first success wins per row; per-source timeout 10 s; size cap
  on the body; no redirects to other hosts.
- Id mapping as DATA (`idmap.json` embedded): public model name -> list of
  `{harness, id}` (e.g. claude-haiku-5-5 for claude-code, gpt-6-luna for codex,
  composer-2.5 for cursor, flash/pro for antigravity). Unmapped public names are
  dropped; session ids with no row are reported as uncovered. No prefix guessing.
- Cache: `scores.json`, atomic write (tmp + rename), never replaced by an empty or
  failed fetch; lock file against concurrent refresh; `DOWNSHIFT_SCORES=off`
  makes everything a no-op.
- CLI `downshift models scores [--refresh] [--status] [--quiet] [--json]`:
  `--status` prints cache age/source/count and, per harness, the session models
  (via `core.ResolveSession...` as the hooks do) with their score or "uncovered".
- Registers `core.ScoreHook` reading the cache (no network).
Tests with `httptest` for each source parser (fixture rows incl. Claude 5.5),
id map table test, empty-response safety, atomic write, concurrent refresh lock,
status coverage output. Header on new Go files.
