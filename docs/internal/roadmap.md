# Roadmap — Downshift (execução interna)

**Atualizado:** 2026-10-23  
**Autor:** Tiago Vilas Boas  
**Fonte pública:** [`ROADMAP.md`](../../ROADMAP.md) — este doc detalha fases, pontos e dependências por trilha.

---

## Premissa após a semana de 20-23/10

Duas descobertas reposicionam o roadmap:

1. **O exit-2 do hook não entrega valor no backend em uso (A-P7).** O contrato
   oficial do PreToolUse só bloqueia quando o Gateway controla a aprovação. Com
   `agent.acp_backend = claude` (o setup normal) a chamada já está aprovada: o
   hook registra mas não para nada. Quem entrega ganho real no uso diário é o
   **hook do Claude Code reescrevendo o comando** (`CanRewriteShellCommand`, 5/5
   honrados) e o **compactor**. A Trilha A deixa de ser "shell-wrap advise" e
   vira **compactor-first**: o advise fica como camada inofensiva (fail-open),
   não como fonte de economia.
2. **O que não é código é coleta passiva, não tarefa.** As linhas de dogfood
   (payloads child-end, control group, feedbacks) não "se fazem" — acontecem com
   uso. Ficam listadas como coleta, fora do caminho crítico.

## Estado atual (main, 2026-10-23)

| Área | Status |
|------|--------|
| `go test ./...` | ✅ verde |
| Dashboard Wave 1 (#453, #454, #451) | ✅ mergeado (#458) |
| Dashboard E2E `?e2e=1` (#470) | ✅ mergeado |
| Git diff/show compressor (#459) | ✅ mergeado |
| `go test -v` subtestes limpos (#465) | ✅ mergeado |
| DCI shared runner (#468) | ✅ mergeado |
| Doc drift + CI gate G1-G3 (#460) | ✅ mergeado |
| Setup remove linha morta do `hooks.json` A-P8 (#471) | ✅ mergeado |
| Downshift-site Next.js M1–M4 | ✅ mergeado |
| Downshift-site M5+M6 | 🔲 aguardando rate limit Vercel (~17h UTC 23/10) |

---

## Trilha A — Shell wrap (encerrada como fonte de economia)

**Fonte:** `docs/spikes/kirocrew-shell-policy.md`  
**Veredito após A-P7:** o exit-2 só morde no backend kiro-cli não-trusted. No
backend em uso (claude) o hook não bloqueia — o ganho vem do rewrite do Claude
Code e do compactor. O advise permanece ligado por ser fail-open e inofensivo,
mas **não é mais um workstream ativo**. A expansão real de valor migrou para a
Trilha C (compactor).

| Fase | Pts | O que é | Status |
|------|-----|---------|--------|
| **A-P1** | 2 | KiroCrew: exit 2 com o comando já embrulhado, só nas famílias comprimíveis. Passthrough e `git status` allow. `DOWNSHIFT_EXEC=0`/`"exec":false` allow. | ✅ mergeado (#461, #462, #464) |
| **A-P6** | 1 | Auditar exit 0 em todo erro, panic e timeout de `downshift kirocrew` | ✅ auditado (2026-10-22) — já coberto pelos testes existentes |
| **A-P7** | 1 | Contrato do PreToolUse: não impede chamada já aprovada pelo kiro-cli | ✅ lido (2026-10-22) — ver achado abaixo |
| **A-P8** | 1 | `setup kirocrew` remove a linha morta "Downshift KiroCrew" do `hooks.json`; o hook que vale fica em `agent.kiro_hooks` | ✅ mergeado (#471) |
| **A-P2** | 1 | Prova ao vivo do bloqueio no backend kiro-cli | ⛔ won't-fix no backend claude — inalcançável no setup em uso (A-P7). Reabrir só se migrar para kiro-cli não-trusted |
| **A-P3** | 2 | Contar bloqueios do advise na telemetria | ⛔ won't-fix — conta um evento que quase nunca dispara no backend em uso |
| **A-P4** | 2 | Parsear token do child-end num `postToolUse` real | 🔲 coleta — depende de payload capturado ao vivo (ver Trilha D) |
| **A-P5** | 3 | `updated_input.command` no Cursor e no Codex (rewrite real, não exit-2), um harness por mudança | 🔲 depende de payload ao vivo por harness. **Este sim entrega valor**: Cursor/Codex reescrevem de verdade |

**Achado de 2026-10-22: o hook `kirocrew` só atua no backend kiro-cli.** Com
`agent.acp_backend = claude` em `~/.kiro/crew/config.json`, as sessões normais do
KiroCrew rodam no backend do Claude: o shell passa pelo hook do Claude Code, que
reescreve o comando no lugar (`CanRewriteShellCommand`) e já compacta. O caminho
exit 2 (A-P1) só dispara em sessões do backend kiro-cli (as que escrevem em
`~/.kiro/crew/audit.log`; o `tool_name` real é `shell`, não `execute_bash`).

**Achado de 2026-10-22 (A-P7): contrato oficial do PreToolUse no Crew.** Fonte:
[kiro.dev/docs/crew/capabilities/hooks](https://kiro.dev/docs/crew/capabilities/hooks/)
(atualizado 12/10/2026). Trecho exato: *"A PreToolUse hook can block only when the
Gateway controls approval. If Kiro CLI already approved the call, the hook reports
the event but cannot stop it."* O exit 2 só tem efeito quando o Gateway ainda
controla a aprovação (modo interativo / não-trusted). A flag `CanAdviseShellWrap`
permanece ligada (fail-open e inofensiva).

**Fora do escopo desta trilha:** routing `use_subagent` sem `model`, process reaping, log trimming, promoção do Haiku 4.5.

---

## Trilha B — Dashboard

| Fase | Pts | O que é | Depende de | Status |
|------|-----|---------|------------|--------|
| **B-W1** | 6 | #454 URL bidirecional + #453 rótulo + #451 doc (bundle reembarcado) | — | ✅ mergeado (#458) |
| **B-W2** | 3 | DASH-2: harness E2E Playwright com flag `?e2e=1` — sensor computacional para #456 (auto-refresh re-render + Motion instável) | — | ✅ mergeado (#470) |
| **B-W3** | 3 | DASH-1: campo `tokens` embutido em `/api/sessions` — `routing: TokenSaving` + `compactor?` por sessão; aba Tokens mostra donut de % real ao expandir a sessão. Decisão: campo inline (KISS/YAGNI, sem endpoint separado); front wired com `TokensSummary` | ✅ #476 API + #477 front |

---

## Trilha C — Compactor

| Fase | Pts | O que é | Depende de | Status |
|------|-----|---------|------------|--------|
| **C-1** | 8 | Git diff/show compressor com safety gate fail-open | — | ✅ mergeado (#459) |
| **C-2a** | 3 | Linters de pass-through para compresses: eslint, ruff, mypy, golangci-lint. Os compressors já existiam mas eram sombreados por detectores genéricos (ripgrep/tsc) e os detectores `is*` não eram multiline; movidos para antes do TSC, detectores com `(?m)`, guarda rejeita ANSI (fail-open), fixtures clean+preserve e `expectedFormat` por família | corpus clean+preserve por família | ✅ linters compactam (eslint 304→189, ruff 174→112, mypy 214→113, golangci 167→132) |
| **C-2b** | 3 | Builds de pass-through para compresses: go build, cargo build, dotnet build. Estavam sombreados por ripgrep (`.go:L:C:`) e cargo-test (danger word `error`); cargo/dotnet movidos para antes do cargo-test, go build para depois dos linters (golangci vence o mesmo `.go:L:C:`) e com guarda contra go vet (`# [package]`) e sufixo `(linter)`; `isCargoBuild` exige marcador de build (`Compiling`/`aborting due to`) para não roubar fragmentos de erro de cargo-test; detector multiline para go build; guarda rejeita ANSI (fail-open); fixtures clean+preserve e `expectedFormat` por família; doc `harness-matrix.md` para `compresses` | C-2a | ✅ builds compactam (go 128→113, cargo 164→82, dotnet 171→107); cargo-test/go-vet/golangci preservados |
| **C-3** | 3 cada | Novas famílias citadas nas steerings sem compressor: `bazel`, `nx`, `psql` — avaliar volume real via `downshift learning` antes de implementar | dados de uso real | 🔲 |

| **C-4** | 3 | `go test -v` com subtestes que passam sem log: Go imprime o `--- PASS` do pai e depois as linhas indentadas dos subtestes; antes qualquer subteste deixava o log inteiro passar (33 KB no `execwrap -v`). FAIL, linha de log, `=== PAUSE/CONT` e multi-pacote continuam byte-idênticos | achado ao testar o hook ao vivo | ✅ PR #465 (transcrição real: 4071 → 171 bytes) |
| **C-5** | 2 | Medir quanto do `go test -v` real ainda cai em pass-through (subteste com `t.Log`, `-parallel`, multi-pacote) antes de abrir mais parsers; hoje só há 1 amostra | `downshift learning` com dados de uso | 🔲 |

**Tradeoff conhecido de C-1:** diffs com `error`/`fail` em qualquer linha caem em pass-through (conservative). Decisão aberta: afrouxar o gate para comprimir mesmo quando danger word aparece só em linha `+`/`-` (já preservada verbatim)?

---

## Trilha D — Roteamento e telemetria

**Código (ativo):**

| Fase | Pts | O que é | Depende de | Status |
|------|-----|---------|------------|--------|
| **D-1** | 5 | OpenRouter part 4: shadow routing para gateway rows | parts 1–3 mergeadas | 🔲 |
| **D-2** | 5 | OpenRouter part 5: wiring OpenCode com id `openrouter/<slug>` | D-1 | 🔲 |
| **D-3** | 5 | DCI hook via shared runner (layering rule) | — | ✅ mergeado (#468) |

**Coleta passiva (não é tarefa — acontece com uso, não se "faz"):**

| Fase | O que falta | Como se resolve |
|------|-------------|-----------------|
| **D-4** | Child-token payloads: KiroCrew/Cursor/Codex/Antigravity (infra pronta, falta payload real) | usar os harnesses; capturar 1 payload ao vivo por harness |
| **D-5** | Control group 20+ tasks para `downshift report` (hoje 14) | dogfood até somar 20+ |
| **D-6** | Economy mode: 974 feedbacks pendentes, 0 rotulados | sessão de rotulagem dedicada |

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

## Trilha F — Documentação e drift

> Fonte: análise de 22/10/2026 comparando `docs/architecture/`, `docs/guides/harness-engineer.md`
> e `docs/design/architecture.md` contra o código em main.

### F-1: `docs/archive/agentic-bdi.md` — drift crítico (2 pts)

O doc descreve T0-T4 com T4 "cravado em Opus", risco força tier ≥ T3, Naive Bayes como
classificador, learning loop que altera o próximo roteamento, e o hook como `dsmon-hook`
com resposta `{allow, model_override}`.

O código tem: 3 tiers (small/mid/frontier), risco é piso dentro dos 3 tiers, regex
decide e o Bayes opina depois, learning guarda forma de saída do compressor e treino de
rota só entra com rótulo revisado fora do hook, e o hook é `downshift <harness>`.

**Fix:** reescrever o doc para refletir o código real ou mover para `docs/archive/`.
Não corrigir parcialmente — o doc atual é enganoso para quem contribui.

| Ação | Pts |
|------|-----|
| F-1a: ~~Reescrever `agentic-bdi.md`~~ → arquivado em `docs/archive/agentic-bdi.md`; doc já se marcava como design antigo e apontava para `architecture.md` como fonte viva; reescrever manteria drift | ✅ #475 |

### F-2: `docs/archive/project-architecture.md` — drift moderado (1 pt)

Referenciado por `harness-engineer.md` como fonte de arquitetura de produção. Descreve
tipos `Session`, `Policy`, `ShouldEscalate` que não existem no núcleo (a decisão é
`Decision` + `PlanSession`).

| Ação | Pts |
|------|-----|
| F-2a: ~~Atualizar ou arquivar `project-architecture.md`~~ → arquivado em `docs/archive/project-architecture.md`; estruturas Session/Policy/ShouldEscalate descritas no doc divergem do código real (Session usa Unknown/QuotaStatus/CreditHeld; Policy usa Version/MinTier/MinTierBy*; ShouldEscalate não existe); doc já apontava para `docs/design/architecture.md` como fonte viva | ✅ #XXX |

### F-3: `docs/guides/harness-engineer.md` — afirmações incorretas de produção (2 pts)

Marca Cursor, Codex e KiroCrew como "✅ Produção" em roteamento. O estado real:
- Cursor: roteamento funciona, shell wrap sem payload capturado, usage source bloqueada
- Codex: roteamento funciona, shell wrap sem payload, usage none
- KiroCrew: advise-only (shell rewrite sem updated_input), usage none

Aponta para `docs/archive/agentic-bdi.md` e `docs/archive/project-architecture.md`
(dois docs com drift crítico).

| Ação | Pts |
|------|-----|
| F-3a: Tabela por nível de evidência real em `harness-engineer.md` (Claude Code=rewrite nativo; KiroCrew=advise-only pós-A-P7; Cursor/Codex=routing ok, sem payload; OpenCode/Antigravity=experimental) | ✅ #475 |
| F-3b: ~~Substituir links para `agentic-bdi.md`/`project-architecture.md` por `docs/design/architecture.md`~~ → concluído; roadmap.md e roadmap-critico.md atualizado para refletir docs arquivados | ✅ #XXX |

### F-4: `docs/design/architecture.md` — 3 gaps internos (3 pts)

O doc tem known gaps abertos que divergem do código atual:

| Gap | Doc diz | Código tem |
|-----|---------|-----------|
| Telemetria grande | "31 non-test files" | 40 arquivos não-test hoje |
| Import graph | "No test or CI step" | `internal/archtest.TestImportGraph` existe e roda no `go test` |
| Compactação | "measure-only PostToolUse" + "`downshift context compress`" | `internal/shellwrap`, `internal/execwrap`, `AdviseAdapter` existem; rewrite de shell do CC ativo |
| Pacotes ausentes no grafo | Grafo de 19/10/2026 não inclui `accounting`, `execwrap`, `shellwrap`, `replay`, `api`, `cursorbridge`, `gateway` | Todos existem em main |
| `internal/accounting` | ADR Proposed, `cmd/downshift` não importa | Pacote existe, ADR continua Proposed, não integrado em `stats` |
| `internal/gateway` | "HTTP gateway ainda é futuro" | `internal/gateway` já é porta para OmniRoute e OpenRouter |

| Ação | Pts |
|------|-----|
| F-4a: Atualizar contagem de telemetria (31→40) e nota do import graph sensor | 1 |
| F-4b: Atualizar seção de compactação para incluir shellwrap/execwrap/AdviseAdapter | 1 |
| F-4c: Atualizar grafo de imports (adicionar accounting, execwrap, shellwrap, replay, api, gateway) | 1 |

### F-5: Sensor de import graph — sem CI gate (3 pts)

`docs/design/architecture.md` diz "no test or CI step" mas `internal/archtest.TestImportGraph`
existe. O gap real: o teste existe mas **não está no CI como gate bloqueante para imports
proibidos** (ex: `internal/core` importar `telemetry`). Um import proibido passa verde.

| Ação | Pts |
|------|-----|
| F-5a: Verificar se `archtest.TestImportGraph` está na suite padrão e bloqueia imports proibidos | 1 |
| F-5b: Se não bloqueia: adicionar ao CI como gate com casos específicos dos invariantes do architecture.md | 2 |

---

## Ordem de execução recomendada para trilha F

```
F-4a (contagem + sensor) — trivial, 30min, só edição de doc
F-3b (substituir links) — trivial, remove referência a docs ruins
F-4c (grafo de imports) — 1h, atualiza o ascii no architecture.md
F-2a (project-architecture.md) — 2h, archivar ou reescrever
F-4b (seção de compactação) — 1h, após extração shellwrap concluída
F-5a/b (CI gate) — 2h, após confirmar o que archtest cobre hoje
```

**Obs:** F-2a pode ser feito junto com F-3b (ambos tocam referências a docs antigos).

---

## Caminho crítico recomendado

A Trilha A saiu do caminho crítico (A-P7: exit-2 não entrega valor no backend em
uso). O valor real está no compactor e no dashboard.

```
✅ Entregue (23/10/2026):
  C-2a (linters)  ─ eslint/ruff/mypy/golangci-lint compactam (#473)
  C-2b (builds)   ─ go/cargo/dotnet build compactam (#474)
  B-W3            ─ campo tokens por sessão + donut no front (#476 + #477)
  F-1a/F-3a       ─ docs corrigidos pós-A-P7 (#475)

Próximo (código, entrega direta):
  C-5     — medir pass-through restante do go test -v (coleta)
  A-P5    — Cursor/Codex updated_input (payload ao vivo necessário)

Quando a Vercel liberar:
  E-M5 + E-M6 (SEO + deploy do site)

Coleta passiva, em paralelo ao uso:
  D-4 (child-token payloads) · D-5 (control group) · R-1/R-2 (dogfood)
```

**Próxima ação de maior valor:** A-P5 (Cursor/Codex rewrite real, com payload capturado)
ou E-M5/M6 (site) quando a Vercel liberar.

---

## Decisões abertas

| # | Decisão | Opções |
|---|---------|--------|
| 1 | Gate do git diff/show: danger word em linha `+`/`-` → pass-through ou comprimir? | Conservador (atual) · Afrouxar para linhas de mudança |
| 2 | DASH-1 API shape: como agregar tokens por sessão? | Campo `tokens` em `/api/sessions` · `GET /api/tokens/aggregate` |
| 3 | `adapt_eval` ainda imprime USD: manter como ferramenta interna opt-in ou alinhar ao "no dollar output"? | Manter (atual) · Alinhar |
