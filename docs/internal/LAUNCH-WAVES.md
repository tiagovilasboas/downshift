# Downshift — plano por ondas (rebrand + OSS launch)

> **Como usar:** uma onda por PR (ou por sprint curto). Marque `[x]` ao concluir.  
> **Não pule ondas com dependência** sem aceitar o risco (tabela abaixo).  
> Contexto: [RENAME.md](../brand/RENAME.md) · hero: [README-HERO.md](../brand/README-HERO.md)

**Estado do repo (baseline):** `tiagovilasboas/downshift` · CLI `downshift` · licença **Apache 2.0** (desde 2026-10-06) · beta ([BETA-EXIT.md](../BETA-EXIT.md)).

---

## Visão das ondas

| Onda | Nome | Objetivo | Depende de |
|------|------|----------|------------|
| **0** | Fundação docs | Arquivos OSS que faltam; tirar ruído interno do público | — |
| **1** | Posicionamento | GitHub + README hero (ainda no repo atual) | 0 |
| **2** | Docs técnicas | Arquitetura, contrib, enxugar over-doc | 0 |
| **3** | Estado local | `~/.downshift` + compat; `downshift doctor` | — (paralelo a 1–2) |
| **4** | Rename técnico | `go.mod`, URLs, goreleaser, install | 1, 3 |
| **5** | Beta exit | P2/P3 ou ajuste de promessas | 1 |
| **6** | Launch | Release, anúncios, comunidade | 4, 5 (mínimo) |
| **7** | Pós-launch (30d) | Crescimento, adoção, open core | 6 |

---

## Onda 0 — Fundação docs (P0)

**Meta:** qualquer visitante entende licença, roadmap e governança sem ler `BACKLOG.md`.

| ID | Task | Arquivos / ação | Done |
|----|------|-----------------|------|
| W0-1 | Criar `COMMERCIAL.md` na raiz (BSL: o que é uso permitido, comercial, contacto, 2030→Apache) | `COMMERCIAL.md` | [x] |
| W0-2 | Criar `ROADMAP.md` público (3–6 meses; sem handoff de agentes; link BETA-EXIT) | `ROADMAP.md` | [x] |
| W0-3 | Criar `GOVERNANCE.md` (maintainer, releases, semver, como virar maintainer) | `GOVERNANCE.md` | [x] |
| W0-4 | Criar `docs/ARCHITECTURE.md` (1 diagrama: harness → adapter → core → catalog; não é gateway HTTP hoje) | `docs/ARCHITECTURE.md` | [x] |
| W0-5 | Mover conteúdo interno para `docs/internal/` **ou** repo privado; deixar stub com link | `docs/BACKLOG.md`, `docs/NEXT-STEPS.md` | [x] |
| W0-6 | README: link para COMMERCIAL, ROADMAP, ARCHITECTURE no índice de docs | `README.md` (só links) | [x] |
| W0-7 | `SECURITY.md`: email de report explícito (não só “ver GitHub”) | `SECURITY.md` | [x] |

**PR sugerido:** `docs: OSS foundation (commercial, roadmap, governance, architecture)`

**Exit:** visitante encontra licença + roadmap sem `BACKLOG.md`.

---

## Onda 1 — Posicionamento (P0)

**Meta:** em 30s no GitHub: **Downshift = model router determinístico** (hooks hoje).

| ID | Task | Arquivos / ação | Done |
|----|------|-----------------|------|
| W1-1 | Aplicar hero **Versão A** de `README-HERO.md` (substituir H1 + primeiras seções) | `README.md` | [x] |
| W1-2 | Título README: `# Downshift` (subtítulo “formerly harness-downshift” uma linha) | `README.md` | [x] |
| W1-3 | Seção **When to use Downshift** vs LiteLLM / OpenRouter / chamada direta (honesta) | `README.md` ou `docs/WHEN-TO-USE.md` | [x] |
| W1-4 | GitHub **description** + **topics** (ver RENAME.md §17) | UI GitHub | [x] |
| W1-5 | Social preview: upload **`docs/brand/downshift-github-social-preview.png`** (1280×640; source SVG alongside) | Settings → General → Social preview | [ ] |
| W1-6 | Atualizar `llms.txt` com nome Downshift + one-liner model router | `llms.txt` | [x] |
| W1-7 | `docs/pt/README.md`: alinhar nome e primeira dobra | `docs/pt/README.md` | [x] |

**PR sugerido:** `docs: reposition as Downshift model router`

**Exit:** description + README hero alinhados (repo renomeado para `downshift` na onda 4).

---

## Onda 2 — Curadoria de documentação (P1)

**Meta:** menos “manual do moat”; mais uso + contrib.

| ID | Task | Arquivos / ação | Done |
|----|------|-----------------|------|
| W2-1 | Condensar `CAPABILITY-ROUTER-V2.md` → ~150 linhas “Vision + status experimental”; mover resto para `docs/internal/` ou apêndice colapsado | `docs/CAPABILITY-ROUTER-V2.md` | [x] |
| W2-2 | Criar `docs/contrib/classifier.md`; mover detalhe de pesos/margem de `docs/pt/02-classificador.md` | `docs/pt/02-classificador.md`, novo arquivo | [x] |
| W2-3 | Índice `docs/README.md` (user / contrib / design / brand) | `docs/README.md` | [x] |
| W2-4 | Manter `router-generalization.md` em `docs/design/`; link no ROADMAP como “rigor de eval” | `ROADMAP.md` | [x] |
| W2-5 | `CONTRIBUTING.md` + `AGENTS.md`: marca Downshift, URLs atualizadas quando souber | `CONTRIBUTING.md`, `AGENTS.md` | [x] |
| W2-6 | `CHANGELOG.md`: entrada “Repositioning; docs structure” | `CHANGELOG.md` | [x] |

**PR sugerido:** `docs: trim v2 doc and split user vs contributor classifier docs`

---

## Onda 3 — Estado local e DX (P1)

**Meta:** migração suave `~/.harness-downshift` → `~/.downshift` antes do rename.

| ID | Task | Arquivos / ação | Done |
|----|------|-----------------|------|
| W3-1 | Helper `StateDir()` com precedência: `DOWNSHIFT_STATE_DIR` → `~/.downshift` → legacy `~/.harness-downshift` | novo `internal/paths` ou `internal/config` | [x] |
| W3-2 | Refatorar paths: telemetry, catalog user override, session-models, loop-events, weights | `internal/telemetry/telemetry.go`, `internal/catalog/catalog.go`, `internal/core/session.go`, `internal/routingv2/training/events.go`, `internal/server/server.go`, `cmd/downshift/main.go` | [x] |
| W3-3 | Comando `downshift doctor` (effective state dir, catalog source, binary version) | `cmd/downshift/` | [x] |
| W3-4 | Documentar migração manual + env em `docs/CONFIG.md` | `docs/CONFIG.md` | [x] |
| W3-5 | `install.sh`: mensagem pós-install com state dir | `install.sh` | [x] |
| W3-6 | Testes: legacy dir only, new dir only, env override | `*_test.go` | [x] |
| W3-7 | `scripts/smoke-test.sh` usar `DOWNSHIFT_STATE_DIR` | `scripts/smoke-test.sh` | [x] |

**PR sugerido:** `feat: configurable state dir with legacy fallback`

**Exit:** usuários beta não perdem `events.jsonl` ao atualizar.

---

## Onda 4 — Rename técnico (P0 para launch “oficial”)

**Meta:** `github.com/tiagovilasboas/downshift` + module path novo.

| ID | Task | Arquivos / ação | Done |
|----|------|-----------------|------|
| W4-1 | `go.mod` → `github.com/tiagovilasboas/downshift`; replace imports em massa | `go.mod`, `**/*.go` | [x] |
| W4-2 | `install.sh` `REPO=tiagovilasboas/downshift` | `install.sh` | [x] |
| W4-3 | `.goreleaser.yml` release name, header/footer URLs | `.goreleaser.yml` | [x] |
| W4-4 | Badges e raw URLs no README, SECURITY, docs | vários `.md` | [x] |
| W4-5 | `LICENSE` Licensed Work: Downshift | `LICENSE` | [ ] |
| W4-6 | `orchestration/pyproject.toml` → `downshift-orchestration` (opcional mesma onda) | `orchestration/` | [ ] |
| W4-7 | **GitHub:** Settings → Rename repository → `downshift` | GitHub UI | [x] |
| W4-8 | Release note “Harness Downshift is now Downshift” (texto em RENAME.md §23) | GitHub Release | [x] |
| W4-9 | Atualizar `docs/brand/RENAME.md` status para “executed” + data | `docs/brand/RENAME.md` | [x] |

**PR(s):** código primeiro; rename GitHub no merge day.

**Exit:** `go install github.com/tiagovilasboas/downshift/cmd/downshift@latest` funciona.

---

## Onda 5 — Beta exit (P0/P1 misto)

**Meta:** remover badge beta **ou** deixar beta com promessas alinhadas à evidência.

| ID | Task | Owner | Done |
|----|------|-------|------|
| W5-1 | P2.3: ≥2 pessoas exportam `downshift stats --export` (issue template) | community | [ ] |
| W5-2 | P2.4: README tabela multi-fonte + data | docs | [ ] |
| W5-3 | P3.4: ≥50 eventos usage ligados a decisões | dogfood | [ ] |
| W5-4 | P3.5: `stats` com Real provider cost não zero | dogfood | [ ] |
| W5-5 | P3.7: preencher `docs/billing-comparison-template.md` uma semana | dogfood | [ ] |
| W5-6 | P1.6: Cursor Pro/Ultra evidência em `docs/HARNESS-MATRIX.md` | dogfood | [ ] |
| W5-7 | Habilitar `claude-code-subagent-stop` no settings maintainer + doc | dogfood | [ ] |
| W5-8 | **Decisão:** se P2/P3 incompletos → README “beta” + BETA-EXIT link; se completos → remover beta | TL | [ ] |

**Issue GitHub sugerida:** `P2.3 — Collect external stats exports (beta exit)`

---

## Onda 6 — Launch (P0)

**Meta:** release tag + distribuição + primeiro ciclo de conteúdo.

| ID | Task | Done |
|----|------|------|
| W6-1 | Tag semver (ex. `v0.2.0` ou `v1.0.0-beta.2`) + Goreleaser | [x] |
| W6-2 | GitHub Discussion pinned: anúncio rename + COMMERCIAL link | [x] [#55](https://github.com/tiagovilasboas/downshift/discussions/55) — pin na UI se ainda não |
| W6-3 | Habilitar **Discussions** (Q&A, Show and tell) | [x] |
| W6-0 | CI fast (`ci`) + nightly `ci-full`; public repo without dataset gates (eval in labs) | [x] |
| W6-4 | Dev.to: artigo problema (subagent cost) — não propaganda | [ ] |
| W6-5 | HN: Show HN (técnico, link ARCHITECTURE + honest limits) | [ ] |
| W6-6 | LinkedIn + X (versões RENAME.md §23) | [ ] |
| W6-7 | `examples/` mínimo: `hook-input.json` por harness | [x] |
| W6-8 | Publicar imagem `ghcr.io/tiagovilasboas/downshift` + doc (P1 se atrasar) | [ ] |

---

## Onda 7 — 30 dias pós-launch (P1/P2)

| ID | Task | Semana | Done |
|----|------|--------|------|
| W7-1 | Post “emission vs honored” + link evidence docs | +1 | [ ] |
| W7-2 | Post outcome metrics: link [benchmark/REPORT.md](../benchmark/REPORT.md); full suite só em downshift-labs | +1 | [ ] |
| W7-3 | Call for misroutes (template já existe) | +1 | [ ] |
| W7-4 | `docs/ADOPTERS.md` (opt-in) | +2 | [ ] |
| W7-5 | Reservar npm scope `@downshift` (sem SDK ainda) | +2 | [ ] |
| W7-6 | Vídeo ou GIF `dsmon` no README | +3 | [ ] |
| W7-7 | Revisar ROADMAP com feedback de issues | +4 | [ ] |
| W7-8 | ~~Avaliar licença~~ → **Apache 2.0** (2026-10-06, ver docs/RELICENSE.md) | +4 | [x] |

---

## Backlog open core (pós-onda 7)

Não bloqueia launch; ordem sugerida:

| ID | Task | Notas |
|----|------|--------|
| OC-1 | Repo privado `downshift-labs` (eval, training ops, BACKLOG autônomo) | [x] 2026-10-06 · `ci-eval` |
| OC-2 | HTTP gateway sketch (`cmd/downshift serve` ou repo separado) | [ ] |
| OC-3 | Cloud / analytics (BSL commercial) | [ ] |
| OC-4 | Enterprise: SSO, audit retention | [ ] |

---

## Métricas (acompanhar na Onda 6+)

| Métrica | Como medir |
|---------|------------|
| Installs | GitHub release downloads + `install.sh` (proxy logs se houver) |
| Ativação | Issues/Discussions com “doctor output” |
| Qualidade routing | Issues `misroute:` fechados com teste em `classifier_edge_test.go` |
| Comunidade | Contributors únicos, PRs adapter |
| Beta exit | Exports P2 + usage P3 |
| Conversão futura | Inbound “commercial license” (COMMERCIAL.md) |

---

## Ordem recomendada de execução (próximos PRs)

```text
1º PR  → Onda 0 (COMMERCIAL, ROADMAP, GOVERNANCE, ARCHITECTURE, internal stubs)
2º PR  → Onda 1 (README hero + WHEN-TO-USE + llms.txt)
3º PR  → Onda 2 (doc trim + docs/README index)     [pode paralelizar com 3º]
3º PR  → Onda 3 (state dir + doctor)               [paralelo]
4º PR  → Onda 4 (go.mod + URLs; rename GitHub no merge)
       → Onda 5 em paralelo (dogfood / community)
5º     → Onda 6 (release + anúncios)
6º     → Onda 7 (conteúdo 30d)
```

---

## Registro de ondas (preencher ao concluir)

| Onda | PR / commit | Data | Notas |
|------|-------------|------|-------|
| 0 | `11d6aba` Apache + OSS foundation | 2026-10-06 | |
| 1 | `d6f0e0b` hero + WHEN-TO-USE | 2026-10-06 | W1-5 social PNG manual |
| 2 | doc trim + contrib/classifier + labs stubs | 2026-10-06 | |
| 3 | `64e2295` internal/paths + doctor | 2026-10-06 | merged com onda 4 |
| 4 | `64e2295`/`660f276` module + GitHub rename | 2026-10-06 | W4-8 via `v0.1.0-beta.6` |
| 5 | template `stats-export` | 2026-10-06 | P2.3 exports ainda 0/2 |
| 6 | `v0.1.0-beta.6` + CI tiers | 2026-10-06 | W6-2/3 manual GitHub UI |
| — | [#57](https://github.com/tiagovilasboas/downshift/pull/57) labs split | 2026-10-06 | REPORT público; gates no labs |
| 7 | | | |

---

harness-downshift by Tiago de Carvalho Vilas Boas  
https://github.com/tiagovilasboas/downshift
