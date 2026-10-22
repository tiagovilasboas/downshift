# Roadmap — Downshift (execução interna)

**Atualizado:** 2026-10-22  
**Autor:** Tiago Vilas Boas  
**Fonte pública:** [`ROADMAP.md`](../../ROADMAP.md) — este doc detalha fases, pontos e dependências por trilha.

---

## Estado atual (main, 2026-10-22)

| Área | Status |
|------|--------|
| `go test ./...` | ✅ verde (61 pacotes) |
| Dashboard Wave 1 (#453, #454, #451) | ✅ mergeado (#458) |
| Git diff/show compressor | ✅ mergeado (#459) |
| Downshift-site Next.js M1–M4 | ✅ mergeado |
| Downshift-site M5+M6 | 🔲 aguardando rate limit Vercel (~17h UTC 23/10) |

---

## Trilha A — Performance: shell wrap agnostic

**Fonte:** `docs/spikes/kirocrew-shell-policy.md`  
**Princípio de design:** cada fase é agnóstica de harness — a camada de decisão
(`shellWrapDecision`) e a de telemetria funcionam para qualquer harness que
implemente a capability. KiroCrew é o piloto; Cursor e Codex entram só após
prova ao vivo por harness.

| Fase | Pts | O que é | Depende de | Status |
|------|-----|---------|------------|--------|
| **A-P1** | 2 | KiroCrew: exit 2 com o comando já embrulhado, só nas famílias que o compactor comprime. Passthrough e `git status` ficam allow. `DOWNSHIFT_EXEC=0` e `"exec":false` mantêm allow. | nada | 🔲 código a escrever |
| **A-P2** | 1 | Um `execute_bash` ao vivo: stderr chega ao modelo e o retry roda embrulhado. Se a volta morrer sem essa linha, reverter o flag | A-P1 instalado | 🔲 evidência necessária |
| **A-P3** | 2 | Contar esses bloqueios na telemetria do hook (quantas vezes o retry disparou por semana) | A-P2 segurou | 🔲 |
| **A-P4** | 2 | Parsear token do child-end num `postToolUse` real do KiroCrew | payload capturado ao vivo (não a P1) | 🔲 |
| **A-P5** | 3 | `updated_input.command` no Cursor e no Codex, um harness por mudança | prova ao vivo por harness + A-P4 | 🔲 |

**Arquivos tocados pela A-P1:**

| Arquivo | Mudança |
|---------|---------|
| `internal/core/capabilities.go` | `CanAdviseShellWrap bool` na struct `KiroCrewCaps` |
| `internal/adapters/kirocrew/kirocrew.go` | `ShellCommand()` para `execute_bash` quando `tool_input.command` está presente |
| `cmd/downshift/shellrewrite.go` | Em `harness_cannot_rewrite` + `CanAdviseShellWrap` + família comprimível → `policyOutput` com `Message = d.Command` |
| `cmd/downshift/hookrun.go` | Shell branch retorna o código de `wire.emit` em vez de sempre 0 |
| `shellrewrite_test.go`, testes kirocrew | Block em `go test ./...`; allow em `git status`, tool não-shell, `DOWNSHIFT_EXEC=0`, Cursor |

**Critério de aceite A-P1:**
1. `go test ./...` num payload KiroCrew: exit 2, stderr contém `compact -- go test ./...`
2. Payload Cursor com o mesmo comando: `{"permission":"allow"}`
3. Sessão ao vivo: `execute_bash` bloqueado mostra a linha stderr ao modelo e o retry roda embrulhado

**Fora do escopo desta trilha:** routing `use_subagent` sem `model`, process reaping, log trimming, promoção do Haiku 4.5.

---

## Trilha B — Dashboard

| Fase | Pts | O que é | Depende de | Status |
|------|-----|---------|------------|--------|
| **B-W1** | 6 | #454 URL bidirecional + #453 rótulo + #451 doc (bundle reembarcado) | — | ✅ mergeado (#458) |
| **B-W2** | 3 | DASH-2: harness E2E Playwright com flag `?e2e=1` — sensor computacional para #456 (auto-refresh re-render + Motion instável) | — | 🔲 |
| **B-W3** | 3 | DASH-1: endpoint agregado de tokens — `tokens: TokenSaving` por sessão em `/api/sessions` para mostrar % real na aba Tokens (hoje mostra taxa de switch) | decisão de API shape | 🔲 |

**Decisão aberta para B-W3:** adicionar `tokens` em `/api/sessions` (cada sessão já retorna) ou criar `GET /api/tokens/aggregate` separado?

---

## Trilha C — Compactor

| Fase | Pts | O que é | Depende de | Status |
|------|-----|---------|------------|--------|
| **C-1** | 8 | Git diff/show compressor com safety gate fail-open | — | ✅ mergeado (#459) |
| **C-2** | 3 cada | Promover compressors de pass-through para compresses: linters (eslint, ruff, mypy, golangci-lint), builds (go build, cargo build, dotnet build) — um PR por grupo | corpus clean+preserve por família | 🔲 |
| **C-3** | 3 cada | Novas famílias citadas nas steerings sem compressor: `bazel`, `nx`, `psql` — avaliar volume real via `downshift learning` antes de implementar | dados de uso real | 🔲 |

**Tradeoff conhecido de C-1:** diffs com `error`/`fail` em qualquer linha caem em pass-through (conservative). Decisão aberta: afrouxar o gate para comprimir mesmo quando danger word aparece só em linha `+`/`-` (já preservada verbatim)?

---

## Trilha D — Roteamento e telemetria

| Fase | Pts | O que é | Depende de | Status |
|------|-----|---------|------------|--------|
| **D-1** | 5 | OpenRouter part 4: shadow routing para gateway rows | parts 1–3 mergeadas | 🔲 |
| **D-2** | 5 | OpenRouter part 5: wiring OpenCode com id `openrouter/<slug>` | D-1 | 🔲 |
| **D-3** | 5 | DCI hook via shared runner (layering rule) + medir economia de contexto injetado antes de mudar default | — | 🔲 |
| **D-4** | — | Child-token payloads: KiroCrew/Cursor/Codex/Antigravity (infra pronta, falta payload real) | evidência por harness (GAP-PAYLOADS) | 🔲 coleta |
| **D-5** | — | Control group 20+ tasks para `downshift report` (hoje 14, precisa 20+) | dogfood | 🔲 coleta |
| **D-6** | — | Economy mode: 974 feedbacks pendentes, 0 rotulados | rotulagem | 🔲 coleta |

---

## Trilha E — Downshift-site (Next.js)

| Fase | Pts | O que é | Depende de | Status |
|------|-----|---------|------------|--------|
| **E-M1** | 3 | Monorepo foundation (Turborepo + pnpm + packages/ui + packages/config) | — | ✅ |
| **E-M2** | 6 | Site marketing: Header, Hero, Features, Problem, Evidence, Install, FAQ, i18n | — | ✅ |
| **E-M3** | 4 | Componentes interativos: routing viz animada + ds-chat + seções restantes | — | ✅ |
| **E-M4** | 5 | Docs app: 20 páginas MDX, sidebar, TOC, dynamic route, SSG | — | ✅ |
| **E-M5** | 3 | SEO polish: metadata dinâmica por página, structured data, Lighthouse ≥ 90 | — | 🔲 amanhã |
| **E-M6** | 4 | Deploy e validação: vercel.json multi-app, CSP hashes, Lighthouse em prod | rate limit Vercel (~17h UTC 23/10) | 🔲 amanhã |

---

## Caminho crítico recomendado

```
Agora:        A-P1 (shell wrap KiroCrew, 2 pts)
              ↓ ao vivo
Evidência:    A-P2 (1 sessão real, não é código)
              ↓ se segurou
Paralelo:     A-P3 (telemetria) + B-W2 (E2E Playwright) + E-M5/M6 (amanhã)
              ↓
Dados:        A-P4 (payload child-end) · D-4 (child-tokens) em dogfood
              ↓ com prova
Expansão:     A-P5 (Cursor/Codex) · D-1/D-2 (OpenRouter) · C-2 (linters)
```

**Próxima ação imediata:** implementar A-P1 (2 pts, sem dependência, toca 4 arquivos).

---

## Decisões abertas

| # | Decisão | Opções |
|---|---------|--------|
| 1 | Gate do git diff/show: danger word em linha `+`/`-` → pass-through ou comprimir? | Conservador (atual) · Afrouxar para linhas de mudança |
| 2 | DASH-1 API shape: como agregar tokens por sessão? | Campo `tokens` em `/api/sessions` · `GET /api/tokens/aggregate` |
| 3 | `adapt_eval` ainda imprime USD: manter como ferramenta interna opt-in ou alinhar ao "no dollar output"? | Manter (atual) · Alinhar |
