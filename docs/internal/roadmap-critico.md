# Roadmap crítico — 5 domínios do Downshift

**Criado:** 2026-10-22  
**Autor:** Tiago Vilas Boas

Este documento é o mapa de críticas por domínio e o caminho mínimo para
resolvê-las. Não é um backlog completo — é o que bloqueia confiança pública.

---

## Pré-leitura: o que o Downshift faz de verdade

Dois domínios mudam execução. Três medem ou acrescentam contexto.

| Domínio | O que muda no mundo | Estado |
|---------|---------------------|--------|
| 1 Roteamento | Escreve o modelo do filho no preToolUse | Em beta |
| 2 DCI | Injeta esboço de código no prompt | Opt-in, sem prova de custo |
| 3 Compactação | Reescreve o comando de shell (compact) | Ativo (CC), parcial (Kiro) |
| 4 Telemetria | Mede tokens e decisões, não executa | Parcialmente cega |
| 5 Adaptadores | Traduz o fio de cada harness | Claude Code provado, resto parcial |

---

## Domínio 1 — Roteamento

### Crítica

O classificador decide tier sem prova de que a decisão é correta fora do
dataset de desenvolvimento. O dataset de desenvolvimento foi usado para tunar
— ele não é evidência independente.

**GAP-SUITE aberto:** 20+ tarefas independentes não estão avaliadas. O
conjunto de 40 tarefas de desenvolvimento está congelado antes de qualquer
tuning, mas ainda é o dataset do mantenedor. A claim "o roteamento funciona"
precisa de avaliação em tarefas que o modelo nunca viu.

**GAP-CONTROL aberto:** 14 tarefas no grupo de controle, precisa 20+ para o
`downshift report` ser publicável. Sem isso o `report` compara pools
desiguais.

**GAP-MEMORY-COUNTS aberto:** o count predictor existe e tem LOO score
inferior ao classificador (34/40 vs 38/40). Não pode ser promovido. A decisão
de se conta-based memory entra como fallback ou feature depende do GAP-SUITE.

### O que fazer

| # | Ação | Tipo | Pts |
|---|------|------|-----|
| R-1 | Coletar 20+ tarefas de controle com `DOWNSHIFT_NO_ROUTE=1` em dogfood real | coleta | — |
| R-2 | Coletar 20+ tarefas independentes fora do dataset de desenvolvimento | coleta | — |
| R-3 | Rodar `downshift report` quando R-1 tiver ≥ 20 controles | evidência | — |
| R-4 | Registrar GAP-GENERALIZATION com resultado R-2 | doc | 1 |

**Bloqueio:** R-1 e R-2 dependem de uso real, não de código. Nenhum PR resolve.

---

## Domínio 2 — DCI (Code Intelligence)

### Crítica

DCI injeta contexto de código opt-in. Dois problemas:

1. **Sem prova de custo:** o `stats` mostra bytes injetados e latência, mas
   não mede se a injeção reduziu buscas subsequentes ou tokens totais da
   sessão. A claim de economizar é hipótese, não medida.

2. **Sem prova de resposta:** não há dado de que o modelo usa o contexto
   injetado de forma diferente do que sem ele. DCI pode ser ruído caro.

3. **O hook roda fora do shared runner** (`dci_hook.go` bifurca por nome de
   harness, fora de `runHookAdapter`). Isso quebra a layering rule e impede
   telemetria unificada.

### O que fazer

| # | Ação | Tipo | Pts |
|---|------|------|-----|
| D2-1 | Mover DCI hook para o shared runner via `HarnessCapabilities` | código | 5 |
| D2-2 | Medir: tokens da sessão com vs sem DCI em ≥ 10 sessões comparáveis | evidência | — |
| D2-3 | Se D2-2 não mostrar redução mensurável em 4 semanas, desativar default e documentar | decisão | 1 |

**Bloqueio:** D2-1 é código imediato. D2-2 e D2-3 são dados.

---

## Domínio 3 — Compactação

### Crítica

O compactor tem dois canais:

1. **`compact` reescreve o comando de shell** — funciona no Claude Code
   (`updated_input.command`). KiroCrew não tem esse canal (exit-0/exit-2 only).

2. **O sensor de resultado da ferramenta só observa** — o compactor vê o
   output depois que acontece. Não há como saber quantos tokens o modelo leu
   de um output não compactado vs compactado, porque a captura só cobre o
   compactado.

**Viés de +15%:** quando o modelo barato gera mais tokens que o modelo caro
teria gerado, `Saved` superestima a economia. Isso não está documentado no
README nem no `stats` output.

**git diff/show gate conservador:** diffs com `error`/`fail` em qualquer
linha caem em pass-through. Diffs de error handling — comuns em Go — nunca
são compactados.

### O que fazer

| # | Ação | Tipo | Pts |
|---|------|------|-----|
| C-1 | A-P1: KiroCrew advise exit-2 (já especificado) | código | 3 |
| C-2 | A-P2: validar ao vivo que o modelo lê o stderr | evidência | — |
| C-3 | Afrouxar gate git diff para danger words só em linhas de contexto (não `+`/`-`) | código | 3 |
| C-4 | Documentar o viés de +15% no `stats --help` e no README | doc | 1 |
| C-5 | Promover compressors de pass-through para compresses (linters) | código | 3/grupo |

**Bloqueio:** C-1 é imediato. C-3 precisa de revisão de safety. C-4 é doc.

---

## Domínio 4 — Telemetria e contabilidade

### Crítica

**A conta de tokens e a conta em dólar são duas contas distintas e
incompatíveis.**

- `telemetry.TokenSavings` conta tokens roteados para um tier mais barato.
  É o que `stats` e `report` publicam. Não inclui compactação.
- A conta em dólar (`Saved = preço_pedido - preço_executado`) só existe no
  ADR e em `adapt_eval.go` (ferramenta interna). Não é publicada no `stats`.
  Tem viés de +15% documentado.

**ADR de accounting em estado Proposed.** Não há pacote `internal/accounting`
no código. A lógica está espalhada em `internal/telemetry` (88 arquivos),
`internal/execwrap` (20 arquivos) e `cmd/downshift` (56+ arquivos).

**GAP-USAGE-50 aberto:** precisa de 50 usage records genuinamente linkados
para o `report` ter estatística de tokens confiável. Hoje: 47 registros
observados, dos quais 38 linkados.

**Logs rotacionados excluídos:** o `stats` ignora os `.bak-*` ao lado do
`events.jsonl`. Há 380k bytes históricos fora da conta.

### O que fazer

| # | Ação | Tipo | Pts |
|---|------|------|-----|
| T-1 | Implementar `internal/accounting` conforme ADR | código | 8 |
| T-2 | `downshift stats` lê de `internal/accounting`, não de telemetria direta | refactor | 5 |
| T-3 | Documentar no `stats` output que a conta é tokens, não dólar, e o viés | doc | 1 |
| T-4 | Coletar 12 usage records adicionais para atingir GAP-USAGE-50 | dogfood | — |
| T-5 | Incluir logs rotacionados no `stats --all-time` flag | código | 2 |

**Bloqueio:** T-1 e T-2 são código, mas grandes. T-3 e T-5 são pequenos e
imediatos. T-4 depende de uso.

---

## Domínio 5 — Adaptadores

### Crítica

Cada harness tem um nível diferente de prova:

| Harness | Roteamento | Shell wrap | Token usage |
|---------|------------|------------|-------------|
| Claude Code | ✅ provado ao vivo | ✅ ativo | ✅ transcript |
| Cursor | ✅ rewrite funciona | ❌ sem payload | ❌ GAP-CURSOR-SOURCE |
| Codex | ✅ rewrite funciona | ❌ sem payload | ❌ GAP-CODEX-HOOK |
| KiroCrew | ⚠️ advise only (A-P1) | 🔲 A-P1 | ❌ GAP-PAYLOADS |
| Antigravity | ✅ provado (P5.5) | ❌ sem shell tool | ❌ GAP-AG-MAPPING |
| OpenCode | ✅ protocolo distinto | ❌ protocolo diferente | ❌ sem seam |

**GAP-WORKFLOW-AGENTS:** agentes criados por `Workflow` do Claude Code não
chegam ao `PreToolUse`. Só `SubagentStart` dispara, sem task text ou modelo.
Downshift não consegue rotear esses agentes. Está documentado mas não
resolvido — depende do vendor.

**GAP-PAYLOADS:** payloads reais de Cursor, Codex, KiroCrew e Antigravity
ainda não capturados para validação de harness. Sem payload real não há como
confirmar `tool_name` de shell nem que `updated_input` é honrado.

### O que fazer

| # | Ação | Tipo | Pts |
|---|------|------|-----|
| A5-1 | Capturar payload KiroCrew com `execute_bash` (dogfood desta sessão) | evidência | — |
| A5-2 | Capturar payload Cursor com shell tool | evidência | — |
| A5-3 | GAP-WORKFLOW-AGENTS: documentar no README e no `doctor` que workflow agents não são roteados | doc | 1 |
| A5-4 | GAP-CURSOR-SOURCE: autenticar native export do Cursor | código | 3 |
| A5-5 | GAP-CODEX-HOOK: native collector path | código | 3 |

---

## Caminho crítico mínimo (o que bloqueia beta exit)

Beta exit tem três pilares: P2 multi-user exports, P3 billing evidence,
comunidade/dogfood. Nenhum deles está verde.

```
Agora (código, sem bloqueio):
  C-1 (A-P1 shell advise KiroCrew)          ← 3 pts
  D2-1 (DCI no shared runner)               ← 5 pts
  T-3 + T-5 (docs viés + logs rotacionados) ← 3 pts
  A5-3 (doc workflow agents)                ← 1 pt

Coleta necessária (sem código resolve):
  R-1 (20+ controles com NO_ROUTE)
  R-2 (20+ tarefas independentes)
  A5-1 (payload KiroCrew execute_bash)
  A5-2 (payload Cursor shell)
  T-4 (12 usage records adicionais)

Com dados em mãos:
  R-3 → downshift report publicável
  C-2 → A-P2 valida shell advise
  T-1 → internal/accounting
  T-2 → stats lê de accounting
  A5-4/A5-5 → Cursor/Codex usage
```

**O que não entra no caminho crítico agora:**
- Conta em dólar publicada (ADR Proposed, viés não corrigido)
- DCI default on (sem prova de custo)
- Promoção count-based memory (LOO score inferior)
- OpenRouter parts 4–6 (nenhum bloqueio de beta exit)

---

## Decisões abertas que o mantenedor precisa tomar

| # | Decisão | Opções |
|---|---------|--------|
| 1 | Publicar conta em dólar no `stats`? | Não enquanto o ADR é Proposed + viés não documentado |
| 2 | DCI default on algum dia? | Só após D2-2 mostrar redução mensurável |
| 3 | Gate git diff: afrouxar danger word em linhas `+`/`-`? | Afrouxar (C-3) vs manter conservador |
| 4 | `internal/accounting`: implementar agora ou depois do GAP-USAGE-50? | Depois (T-4 primeiro) — sem dados suficientes para validar |
| 5 | GAP-WORKFLOW-AGENTS: fix ou documentar como limitação? | Documentar (depende do vendor) |
