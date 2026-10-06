# Harness Downshift → Downshift — migration guide

> **Status:** executed 2026-10-06 (module path, state dir, GitHub rename).  
> **Brand:** Downshift  
> **Repository:** `tiagovilasboas/downshift`  
> **CLI:** `downshift`  
> **Legacy module:** `github.com/tiagovilasboas/harness-downshift` (last tag before rename).

Downshift by Tiago de Carvalho Vilas Boas  
https://github.com/tiagovilasboas/downshift

---

## Goals

| Goal | Detail |
|------|--------|
| Public identity | **Downshift** = marca; **Model Router** = categoria |
| Repo URL | `github.com/tiagovilasboas/downshift` |
| Developer UX | `go install`, `curl install.sh`, and hooks keep working across one deprecation window |
| Data on disk | Existing `~/.harness-downshift/` must keep working until users migrate |
| Package ecosystems | Avoid `npm downshift` (React) and `pip downshift` ([ana-lan/downshift](https://github.com/ana-lan/downshift)) |

## Non-goals (this migration)

- Renaming the CLI binary (`downshift` stays).
- Turning the project into an HTTP AI gateway in the same release (positioning only).
- Project licence: Apache 2.0 (see docs/RELICENSE.md).

---

## Recommended release sequence

Execute in order. Do **not** rename the GitHub repo before a release that supports the new paths.

### Phase 0 — Docs & positioning (safe anytime)

1. Add this file and [`README-HERO.md`](README-HERO.md).
2. Update GitHub **description** and **topics** (no repo rename yet).
3. Pin a Discussion draft: “Harness Downshift is becoming Downshift” (see [Launch messaging](#launch-messaging)).

### Phase 1 — Code: dual-read state directory (one minor release)

Introduce a single source of truth for the user config dir:

| Priority | Path |
|----------|------|
| 1 | `DOWNSHIFT_STATE_DIR` (new) |
| 2 | `~/.downshift/` (new default) |
| 3 | `~/.harness-downshift/` (legacy fallback if it exists and `~/.downshift` does not) |

**Files to touch first (runtime):**

- `internal/telemetry/telemetry.go` — `defaultEventPath`, session hash salt string (keep `harness-downshift-session-v1` for compat or document a v2 salt).
- `internal/catalog/catalog.go` — user `catalog.json` path.
- `internal/core/session.go` — default `session-models.json` (already overridable via `DOWNSHIFT_SESSION_MODELS`).
- `internal/routingv2/training/events.go` — `loop-events.jsonl`, `weights.json`.
- `internal/server/server.go` — any hardcoded `.harness-downshift` paths.
- `cmd/downshift/main.go` — help text and `~/.harness-downshift` strings.
- `cmd/dsmon/main.go`, `cmd/dsmon-hook/main.go`, `scripts/smoke-test.sh`.

Add `downshift doctor` or extend `downshift try --verbose` to print **effective state dir** (guia + sensor for migration).

### Phase 2 — Go module path (breaking for importers)

1. `go.mod`: `module github.com/tiagovilasboas/downshift`
2. Mechanical replace: `github.com/tiagovilasboas/downshift` → `github.com/tiagovilasboas/downshift` (~**150+** import lines across `cmd/`, `internal/`, tests).
3. Tag **`v0.x.0`** with CHANGELOG “Module path change; old path frozen at last harness-downshift tag”.
4. Leave a **README stub** on old module path only if you publish a final tag from old name pointing users forward (optional).

### Phase 3 — GitHub repository rename

1. Settings → Rename repository: `harness-downshift` → `downshift`.
2. GitHub redirects `tiagovilasboas/downshift` → `tiagovilasboas/downshift` (stars/forks/issues preserved).
3. Update in same commit or immediately after:
   - `.goreleaser.yml` — `release.github.name`, header/footer URLs
   - `install.sh` — `REPO=tiagovilasboas/downshift`
   - All badge URLs in `README.md`
   - `raw.githubusercontent.com/.../harness-downshift/...` → `.../downshift/...`

### Phase 4 — User-facing copy & Python orchestration

1. Replace README hero from [`README-HERO.md`](README-HERO.md).
2. Rename Python package (optional separate release):
   - `harness-downshift-orchestration` → `downshift-orchestration` on PyPI
   - `orchestration/src/harness_downshift_orchestration/` → `downshift_orchestration/`
3. One **migration release note** with copy-paste for hooks (unchanged command names).

### Phase 5 — Cleanup (v1.0 or +90 days)

- Deprecation warning when only legacy dir exists (“move to `~/.downshift`”).
- Remove “harness-downshift” from user-visible strings except CHANGELOG history.
- Consider `install.sh` one-liner that rsyncs legacy dir → new dir on first run.

---

## State directory migration (operators)

```bash
# After a build that supports DOWNSHIFT_STATE_DIR / ~/.downshift
mkdir -p ~/.downshift
if [ -d ~/.harness-downshift ] && [ ! -f ~/.downshift/.migrated ]; then
  cp -a ~/.harness-downshift/. ~/.downshift/
  touch ~/.downshift/.migrated
fi
export DOWNSHIFT_STATE_DIR="$HOME/.downshift"   # once implemented
```

Until Phase 1 ships, only `DOWNSHIFT_EVENT_LOG` and `DOWNSHIFT_SESSION_MODELS` override individual files.

---

## Package & registry strategy

| Artifact | Name | Notes |
|----------|------|--------|
| Go module | `github.com/tiagovilasboas/downshift` | After Phase 2 |
| CLI binary | `downshift` | Unchanged |
| Goreleaser project | `downshift` | Already `project_name: downshift` in `.goreleaser.yml` |
| Container | `ghcr.io/tiagovilasboas/downshift` | Prefer GHCR over ambiguous Docker Hub `downshift/downshift` |
| npm (future SDK) | `@downshift/router` or `downshift-router` | **Never** `downshift` ([downshift-js](https://github.com/downshift-js/downshift)) |
| PyPI orchestration | `downshift-orchestration` | **Never** `downshift` ([PyPI project](https://pypi.org/project/downshift/)) |
| User catalog override | `$STATE_DIR/catalog.json` | Today: `~/.harness-downshift/catalog.json` |

---

## Impact matrix (by area)

Counts from repo scan (2026-10-06). Use `rg 'harness-downshift'` before release to refresh.

### A. GitHub & distribution (must change on rename)

| File / area | Kind of change |
|-------------|----------------|
| `README.md` | Title, badges, install URLs, prose (~28 hits) |
| `install.sh` | `REPO`, comments, URLs (8 hits) |
| `.goreleaser.yml` | `release.github.name`, release header/footer (5 hits) |
| `.github/workflows/ci.yml` | Only if workflow references old repo name |
| `llms.txt` | URLs and product name (8 hits) |
| `web/index.html` | Title/meta if published |
| `SECURITY.md` | Security contact URLs (6 hits) |

### B. Go module & imports (Phase 2)

| Pattern | ~Files |
|---------|--------|
| `github.com/tiagovilasboas/downshift/...` imports | All `internal/*`, `cmd/*` tests |
| `go.mod` module line | 1 |
| `install_test.go` | install URL assertions |
| Panic strings `harness-downshift: embedded catalog` | `internal/catalog/catalog.go` |

### C. User config path `~/.harness-downshift` (~45 path references)

| File | Assets under state dir |
|------|-------------------------|
| `internal/telemetry/telemetry.go` | `events.jsonl` |
| `internal/catalog/catalog.go` | `catalog.json` |
| `internal/core/session.go` | `session-models.json` |
| `internal/routingv2/training/events.go` | `loop-events.jsonl`, weights |
| `internal/routingv2/classifier/weights.go` | weights paths |
| `internal/server/server.go` | server static paths |
| `cmd/downshift/main.go` | help strings |
| `cmd/dsmon/main.go`, `cmd/dsmon-hook/main.go` | tail events |
| `catalog.sample.json`, `internal/catalog/catalog.json` | `_doc` comments |
| `docs/session-models.md`, `docs/pt/05-catalogo-e-sessao.md`, `docs/pt/08-telemetria.md` | operator docs |
| `docs/CAPABILITY-ROUTER-V2.md` | `.harness-downshift/weights.json`, repo-local `.harness-downshift/` |
| `scripts/smoke-test.sh` | hermetic HOME |
| `.gitignore` | ignore patterns |
| Brand SVGs under `docs/brand/*routing*.svg` | diagram labels (cosmetic) |

### D. Product naming in docs (no import break)

| File | Notes |
|------|--------|
| `AGENTS.md` | Agent contract title and URLs (10 hits) |
| `CONTRIBUTING.md` | Title, licence blurb |
| `CHANGELOG.md` | Historical + rename entry |
| `docs/BETA-EXIT.md`, `docs/ENGINEERING-LOOP.md`, `docs/GRAPHIFY-INTEGRATION.md` | Cross-links |
| `docs/pt/*.md` | PT docs + attribution footer |
| `docs/CAPABILITY-ROUTER-V2.md`, `docs/CLASSIFIER-SHADOW.md`, `docs/LANGGRAPH-ORCHESTRATION.md` | Technical |
| `benchmark/blind-vitrine.README.md`, `tools/minilm/README.md` | Secondary |
| `.github/ISSUE_TEMPLATE/*.md` | Templates |

### E. Python orchestration

| File | Change |
|------|--------|
| `orchestration/pyproject.toml` | `name`, `description` |
| `orchestration/src/harness_downshift_orchestration/*` | package rename |
| `orchestration/tests/test_graph.py` | imports |
| `orchestration/uv.lock` | regen after rename |

### F. Brand & imagery

| File | Change |
|------|--------|
| `docs/img/hero.svg` | Title text “harness-downshift” → “Downshift” |
| `docs/brand/downshift-*.svg` | Some subtitles still say harness-downshift |
| `docs/brand/README-HERO.md` | New hero (source for README) |
| `docs/brand/downshift-brand-tokens.json` | Verify `productName` field |

### G. Unchanged or low priority

| Item | Reason |
|------|--------|
| CLI name `downshift` | Already correct |
| Env vars `DOWNSHIFT_*` | Already correct |
| `cmd/downshift/` path | Directory name is fine (binary name) |
| Fowler “harness engineering” citations | Keep as concept, not product name |
| `.claude/settings.local.json` | Local only; do not commit |

---

## GitHub settings checklist

After rename to `downshift`:

- [ ] **Description:** `Deterministic open-source model router — right-sized LLMs per workload. Agent hooks, catalog policy, shadow mode, outcome benchmarks.`
- [ ] **Website:** optional — link to README or future docs site
- [ ] **Topics:** `model-routing`, `llm`, `ai`, `cost-optimization`, `agents`, `golang`, `open-source`, `developer-tools`
- [ ] **Social preview (PNG only on GitHub):** upload `docs/brand/downshift-github-social-preview.png` (1280×640; edit `downshift-github-social-preview.svg` and re-export with `rsvg-convert -w 1280 -h 640 …`)
- [ ] Enable **Discussions** for announcement thread
- [ ] Release **v0.x.y: The Downshift rename** with migration notes

---

## Verification checklist (pre-release)

```bash
# From repo root after changes
go test ./...
./scripts/smoke-test.sh
rg -n 'harness-downshift' --glob '!CHANGELOG.md' --glob '!docs/brand/RENAME.md'   # trend to zero in user-facing files
rg -n 'tiagovilasboas/downshift'                                          # should be zero after Phase 2+3
```

Manual:

- [ ] `curl -fsSL .../downshift/main/install.sh | sh` on clean VM
- [ ] `go install github.com/tiagovilasboas/downshift/cmd/downshift@latest`
- [ ] Hook smoke: `downshift claude-code < examples/claude-code-pretooluse-session.json` still rewrites model
- [ ] `dsmon` reads events from effective state dir
- [ ] Legacy install with only `~/.harness-downshift/` still works (Phase 1)

---

## Risk mitigation

| Risk | Mitigation |
|------|------------|
| Broken `go get` old path | Final tag on old module + README redirect; document in CHANGELOG |
| Dead links / SEO | GitHub redirect; update DEV.to/HN posts when convenient |
| Duplicate PyPI/npm names | Use qualified package names only |
| Session hash salt change | Do **not** change salt without major version; breaks session grouping |
| Beta users with hooks | Hooks call `downshift` — no change; only install URL/docs |

---

## Launch messaging

### GitHub Release / Discussion title

**Harness Downshift is now Downshift**

### GitHub body (short)

The project started as a **coding harness** router (Claude Code, Cursor, Codex hooks). The core is broader: **deterministic model routing** for AI workloads—classify task complexity, apply catalog policy, pick tier **without an LLM in the routing loop**.

- **Same CLI:** `downshift`
- **New repo:** `github.com/tiagovilasboas/downshift` (redirect from `harness-downshift`)
- **Harness** is now an **integration category**, not the product name
- State directory: moving to `~/.downshift` (legacy path supported during deprecation)

Full steps: [docs/brand/RENAME.md](RENAME.md).

### LinkedIn (PT)

Apresentamos o **Downshift** (antes Harness Downshift): roteador open source de modelos por complexidade de tarefa—determinístico, local, com benchmarks e shadow mode. O binário continua `downshift`; o escopo agora é model routing para workloads de IA, não só IDEs. Repo: https://github.com/tiagovilasboas/downshift

### X

Harness Downshift → **Downshift**. Same `downshift` CLI. Broader mission: deterministic model routing for AI workloads (not just coding harnesses). OSS beta: https://github.com/tiagovilasboas/downshift

### Dev.to (angle)

Title idea: *Why we dropped “Harness” from Downshift* — Fowler harness engineering as integration story; product = model router; comparison table vs LiteLLM/OpenRouter (honest wedge: hook-local, zero LLM router).

---

## Final target configuration

```text
Brand:              Downshift
Repository:         downshift
CLI:                downshift
Primary category:   Model Router
Secondary category: AI infrastructure
Tagline:            Deterministic model routing for AI workloads.
GitHub description: Deterministic open-source model router — right-sized LLMs per workload. Agent hooks, catalog policy, shadow mode, outcome benchmarks.
State directory:    ~/.downshift (with legacy fallback)
Go module:          github.com/tiagovilasboas/downshift
```

---

## Related

- README hero draft: [`README-HERO.md`](README-HERO.md)
- Brand assets: `docs/brand/downshift-logo-*.svg`, `downshift-brand-tokens.json`
- Collision notes: see rebranding analysis (npm `downshift`, PyPI `downshift`, [downshiftit.com](https://www.downshiftit.com/))
