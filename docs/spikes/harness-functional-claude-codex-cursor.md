# Spike: Downshift funcional em Claude Code, Codex e Cursor

**Status:** proposed (2026-10-24)  
**Autor:** Tiago Vilas Boas  
**Contexto:** KiroCrew já entrega valor diário (roteamento `spawn_run`, compactor via backend Claude ou advise no kiro-cli). Claude Code, Codex e Cursor têm adaptadores maduros no repositório, mas a matriz ainda marca lacunas de **honra**, **fim de child**, **usage** e **prova ao vivo** — o que impede promover confiança e o `doctor` de contar a história certa.

Downshift by Tiago Vilas Boas  
https://github.com/tiagovilasboas/downshift

**Relacionado:** [harness-matrix.md](../reference/harness-matrix.md), [gap-tasks.md](../internal/gap-tasks.md), [2026-10-24-cursor-live](../internal/tasks/2026-10-24-cursor-live/README.md), [shell-wrap-harness-matrix.md](shell-wrap-harness-matrix.md) (parcialmente desatualizado em relação ao código).

---

## Pergunta

O que falta — em código, setup e evidência — para os três harnesses prioritários se comportarem no dia a dia **no mesmo nível de confiança operacional** que o operador já tem no KiroCrew, sem reabrir OpenRouter nem mexer no Kiro?

---

## Definição de “funcional” (contrato do projeto)

| Nível | Significado | Alvo deste spike |
|-------|-------------|------------------|
| **Completed** | Rewrite **honrado** observado; usage e child end ligados; policy/setup documentados | Claude Code (manter; fechar buracos) |
| **Functional** | Rewrite **emitido** em spawn real; conformidade no CI; honra inferida ou não confirmada | Codex e Cursor → subir para Completed onde o harness expõe sinal |
| **Experimental** | Modo policy ou uma observação pontual | KiroCrew — **fora do escopo** (referência de UX, não de código) |

“Funcional no Kiro” para o dono hoje = roteamento visível + compactor no fluxo + telemetria que não mente. Este spike traduz isso em checklist para Claude / Codex / Cursor.

---

## Estado atual (main, 2026-10-24)

### Resumo

| Harness | Status na matriz | Roteamento subagent | Shell compactor in-hook | Child end | Usage / honor |
|---------|------------------|---------------------|-------------------------|-----------|---------------|
| **Claude Code** | Completed | `PreToolUse` Task/Agent, family names | `Bash` → `updatedInput.command` ✅ ao vivo | `SubagentStop` ✅ | Transcript ✅; `PostToolUseFailure` não verificado (GAP-CHILD-FAILURE) |
| **Codex** | Functional | `PreToolUse` Agent/spawn_agent ✅ emitido | `Bash` implementado (#503); **honra shell não provada** | Parser PostToolUse **não verificado** | `UsageSource: none`; honra **inferida** (2026-10-02) |
| **Cursor** | Functional | `preToolUse` Task; cache composer (#547) | `Shell` + advise `beforeShellExecution` (#512); **honra shell não provada** | `subagentStop` link por `subagent_id` ✅ (3.27.12) | Sem usage; honra Task **não confirmada** (P1.6 / GAP-CURSOR-HONOR) |

Capabilities em código: os três têm `CanRewriteShellCommand: true` (`internal/core/capabilities.go`). Spikes antigos que dizem “stub / false” estão **desatualizados** — o gap restante é **prova ao vivo**, não implementação ausente.

### KiroCrew (barra de referência, sem trabalho neste spike)

- Roteamento: block + respawn com modelo explícito (`spawn_run`); batch `tasks[]` roteado por entrada.
- Compactor: rewrite nativo no backend Claude; advise exit-2 no kiro-cli (Trilha A encerrada como fonte de economia no backend em uso).
- Telemetria: sem session id agrupável; child-end ainda depende de payload real (coleta passiva).

---

## Lacunas que impedem “tão bom quanto Kiro”

### Claude Code — hardening, não greenfield

| ID | Lacuna | Impacto no dia a dia | Tipo |
|----|--------|----------------------|------|
| CL-1 | GAP-CHILD-FAILURE: `PostToolUseFailure` sem payload real | Retry/erro de subagent não entra no report | Código + setup + prova |
| CL-2 | GAP-CLAUDE-QUOTA: statusline / `quota.json` não observados ao vivo | Sem janela de quota no fluxo | Setup + prova (bridge já no source) |
| CL-3 | GAP-WORKFLOW-AGENTS: spawns via `Workflow` não passam `PreToolUse` | Buraco de roteamento documentado, não fechado | Doc + possível guard no plugin |
| CL-4 | Baseline em paralelo (primeira mensagem com N Agents) | Holds `HELD_UNCONFIDENT_UNKNOWN_MID` | Já mitigado por settings fallback; medir se ainda dói |
| CL-5 | Binário desatualizado no hook (`dev` sem subcomandos) | Doctor mente; quota/collect não roda | **Operação:** `go install` / release tag |

### Codex

| ID | Lacuna | Impacto | Tipo |
|----|--------|---------|------|
| CX-1 | GAP-CODEX-HOOK: nenhum stdin de hook com `transcript_path` capturado | Baseline/quota live não provados | Prova + possível ajuste adapter |
| CX-2 | Honra só **inferida** (2026-10-02) | Matriz não sobe para Completed | Prova (spawn + modelo executado ou campo upstream) |
| CX-3 | Child end PostToolUse **unverified** | `report` / child line incompletos | Fixture de payload real + teste `runEnd` |
| CX-4 | Shell `Bash` rewrite **unverified live** | Compactor automático igual Claude | Prova 1× `go test` |
| CX-5 | Spawns sem chave `model` no payload (comum) | Depende de `transcript_path` / turn_context | Ligado a CX-1 |

### Cursor

| ID | Lacuna | Impacto | Tipo |
|----|--------|---------|------|
| CU-1 | GAP-CURSOR-HONOR / P1.6: rewrite emitido, executor do child ausente no transcript | Continua Functional | Prova em plano usage-based + build ≥3.27 |
| CU-2 | Task `preToolUse` sem `model` até cache quente | Holds R9 (visto 2026-10-24) | **Mitigação mergeada:** `cursor-touch` + composer cache; falta prova “rewrite_emitted” após warm-up |
| CU-3 | Shell / Task: planos Free e Pro legacy ignoram `updated_input` | Doctor deve avisar | Doc + doctor |
| CU-4 | GAP-CURSOR-SOURCE: export nativo de quota bloqueado (3.24.9+) | Quota routing live impossível hoje | Fora do MVP deste spike |
| CU-5 | Dois harnesses no mesmo spawn (Cursor + cópia claude-code) | Já filtrado por `PayloadMarkers`; revalidar em build atual | Prova curta |
| CU-6 | Shell rewrite **unverified live** | Mesmo que CX-4 | Prova 1× `go test` via agente Shell |

---

## O que **não** entra neste spike

- OpenRouter (parts 4–6) — fila final (dono 2026-10-24).
- KiroCrew, Antigravity, OpenCode plugin, gateway HTTP.
- Promover routing v2 ou scorer aprendido.
- Quota Cursor nativa (GAP-CURSOR-SOURCE) até vendor liberar API externa.

---

## Plano em ondas (Fibonacci, teto 5 por PR)

Ordem: **prova desbloqueia código** onde o payload ainda é UNKNOWN; ondas 2 e 3 podem paralelizar por harness (arquivos diferentes).

### Onda 0 — Baseline operacional (1–2 h, dono + agente)

| # | Pts | Entrega | Pronto quando |
|---|-----|---------|---------------|
| W0-1 | 1 | `downshift doctor --json` nos três harnesses; binário = release ou `go install` do HEAD | Nenhum hook aponta para `dev` sem subcomandos |
| W0-2 | 1 | `downshift setup claudecode\|codex\|cursor --dry-run` → diff revisado → `--yes` | Hooks batem com `setup_targets.go` |
| W0-3 | 2 | Folha de prova única `docs/evidence/harness-functional-2026-10-24.md` (sem prompt/path/conta) | Template com seções CL / CX / CU |

### Onda 1 — Sprint de prova ao vivo (~45–60 min dono)

Executar na ordem abaixo; cada bloco grava só hashes, plan/build, outcomes e field names.

| # | Pts | Prova | Fecha |
|---|-----|-------|-------|
| P-CL | 2 | Claude: Task trivial + Task que downshifta + `go test` via Bash; confirmar `rewrite_emitted`, `SubagentStop`, linha child se visível | CL-5 implícito; baseline Completed reaffirmed |
| P-CX | 2 | Codex: `/hooks` trusted; `spawn_agent` mecânico; capturar stdin hook (redigido) + um `go test` Bash | CX-1, CX-2, CX-4 |
| P-CU | 3 | Cursor 3.27+: abrir sessão (warm cache) → Task mecânico → Shell `go test`; `downshift probe` se algo falhar | CU-2, CU-5, CU-6; input para CU-1 |

**Gate da onda 1:** se Codex/Cursor shell não honrar, ligar **advise** (já wired no Cursor `beforeShellExecution`) e registrar plano — não bloquear release por shell se roteamento Task funcionar.

### Onda 2 — Código guiado por evidência (paralelo por harness)

| # | Pts | Harness | Entrega | Depende |
|---|-----|---------|---------|---------|
| C-CL-1 | 2 | Claude | Registrar hook `PostToolUseFailure` no setup se faltar; parser + teste `runEnd` | Payload CL-1 |
| C-CX-1 | 3 | Codex | Mapear PostToolUse real em `ParseChildEnd` + teste | Payload CX-3 |
| C-CX-2 | 2 | Codex | Confirmar/leer `transcript_path` live em `sessionmodel.go`; teste fixture | Payload CX-1 |
| C-CU-1 | 3 | Cursor | Se após warm cache ainda `allow`+hold: `session-models.json` + slugs Cursor documentados | P-CU falhou roteamento |
| C-CU-2 | 2 | Cursor | Atualizar matriz P1.6 somente se prova mostrar executor model ou refutação clara | P-CU |

Nenhuma mudança em `core.Route` para “forçar” tier: dúvida continua allow (R7/R8/R9).

### Onda 3 — Superfície de produto (confiança tipo Kiro)

| # | Pts | Entrega | Arquivos |
|---|-----|---------|----------|
| H-1 | 2 | **RT-5 / H5:** `doctor` por harness — hooks wired, último `rewrite_emitted` / inferido, shell rewrites contados, aviso plano Cursor | `cmd/downshift/doctor*.go` |
| H-2 | 1 | `install.md` + harness-matrix: três linhas “o que esperar” alinhadas às provas | docs |
| H-3 | 2 | `downshift try` ou doc one-liner por harness (“como saber que está funcionando”) | cmd ou guides |

### Onda 4 — Promoção de status (só com evidência)

| Harness | Promove para | Condição |
|---------|--------------|----------|
| Claude Code | Mantém **Completed** | CL-1 opcional vira checkbox beta-exit |
| Codex | **Completed** se honra observada + child end verificado | CX-2 + CX-3 |
| Cursor | **Completed** só com P1.6; senão **Functional** com doctor explícito | CU-1 ou honest “unconfirmed” |

---

## Caminho crítico recomendado

```
W0 (doctor + setup + template evidência)
  → P-CL ∥ P-CX ∥ P-CU   (dono, mesma tarde)
       → C-* por harness (PRs pequenos)
            → H-1 doctor
                 → matriz + release notes (RL-3)
```

**Máximo valor antes de tag 0.1.4-beta:** W0 + P-CU (Cursor é prioridade de produto com maior incerteza) + H-1. Codex em paralelo. Claude é validação rápida.

**Estimativa total implementação (sem contar tempo do dono):** ~18–22 pts (~4–6 PRs) + 1 h de prova.

---

## Protocolo de prova (copiar para `docs/evidence/harness-functional-2026-10-24.md`)

1. Anotar versões: `claude --version`, Cursor About, `codex-cli --version`, `downshift version`.
2. Antes: `downshift doctor --json` → salvar status checks (sem paths home no commit público se sensível).
3. Durante: uma Task/subagent spawn por harness; anotar `outcome`, `requested_model`, `final_model`, `recommended_model` (grep em events.jsonl por harness).
4. Shell: um `go test ./...` allowlisted; verificar se comando executado contém `downshift compact` (log do harness ou stderr hook).
5. Depois: `downshift report --days=1` — harness aparece com decisões; Cursor deve mostrar >0 eventos `cursor` (problema histórico: 0 cursor / centenas claude-code cópias — hoje filtrado, confirmar).
6. Cursor honor: só afirmar “honored” se campo upstream ou UI mostrar modelo do child; senão marcar “emitido, honra pendente”.

---

## Riscos e trade-offs

| Risco | Mitigação |
|-------|-----------|
| Cursor Free/legacy descarta rewrite | Doctor avisa; matriz já documenta |
| Codex schema muda | GAP-CODEX-REVALIDATE periódico; fixtures versionadas |
| Prova consome crédito | Tasks mecânicas mínimas; uma spawn por harness |
| Duplicar esforço com [cursor-live](../internal/tasks/2026-10-24-cursor-live/README.md) | T4/T7 feitos; este spike cobre honor + shell + doctor transversal |

---

## Touch list (implementação)

| Área | Pacotes / docs |
|------|----------------|
| Adapters | `internal/adapters/claudecode`, `codex`, `cursor` |
| Shared | `cmd/downshift/hookrun.go`, `end.go`, `shellrewrite.go`, `doctor*.go`, `setup_targets.go` |
| Evidência | `docs/evidence/harness-functional-*.md`, atualizar `harness-matrix.md`, `capability-matrix.md` |
| Testes | `runHookAdapter`, `runEnd`, `doctor_test.go`; sem novo cenário no classificador |

---

## Próxima ação sugerida

1. Dono: **W0-1** (binário único no PATH) + **P-CU** com sessão que passe por `sessionStart`/`beforeSubmitPrompt` (cache quente).  
2. Agente: criar **W0-3** template de evidência + PR **H-1** (doctor) em paralelo, sem esperar honor Cursor.  
3. Após P-CX: PR **C-CX-1** só se payload trouxer campos novos.

Quando as três provas estiverem no vault de evidência, revisitar promoção Codex/Cursor na matriz e o texto de release **0.1.4-beta**.
