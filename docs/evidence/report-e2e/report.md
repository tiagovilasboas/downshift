# Report e2e — BLOCK

Captura `2026-10-09T11:07:16Z` em `http://127.0.0.1:7474`, binário `./downshift` gerado `2026-10-09 07:58:08` (depois de `fb55f8a`) e processo `./downshift serve` iniciado `07:58:14`. Sensor: `node --experimental-websocket docs/evidence/report-e2e/live-report.check.mjs` (exit 1). PNGs: `all.png`, `antigravity.png`, `codex.png`, `cursor.png`, `claude-code.png`, `kirocrew.png`, `grok.png`.

Log de compactação ausente: `~/.harness-downshift/context-compactions.jsonl` não existe. `compaction.by_harness` veio `{}`. Nenhuma linha de harness foi inventada.

## Bloqueios

1. **Codex mente janela de 24h.** A aba diz `no codex events in the last 24h` e `0 events`. `events.jsonl` tem 3 eventos nessa janela. Um deles é `2026-10-08T17:57:51Z`, `gpt-5.5`, `final_reasoning_effort=medium`, verdict `UPSHIFT`. A aba não mostra o id nem o effort.
2. **Claude Code mente janela de 24h.** A aba diz `no claude-code events in the last 24h` e `0 events`. O log tem 29 eventos. Exemplo: `2026-10-09T02:02:42Z`, `claude-opus-4-8`, `final_reasoning_effort=medium`.
3. **Cursor conta o rabo da API, não as 24h.** A aba mostra `8 events`, `4↓`, `$0.00` e 4 downshifts. O log tem 43 eventos cursor na janela. Os 8 de `/api/status` são todos `grok-4.7-xhigh` com `final_reasoning_effort=unknown` e `outcome=allow`. O `↓ medium` é a complexity `MEDIUM`. Eventos com effort ficaram de fora, por exemplo `2026-10-09T00:27:58Z` `muse-spark-1.3-high` · `high`.
4. **Bloco agents do cursor nega os eventos da mesma tela.** Os switches listam `grok-4.7-xhigh` e o bloco agents diz `no cursor events in the last 24h`.

Causa: `/api/status` devolve só os últimos 8 decisions. Na captura, os 8 são cursor. O chip trata esse rabo como a janela de 24h.

Repro: abrir `http://127.0.0.1:7474`, clicar em codex, claude-code e cursor, e comparar com `events.jsonl` nas últimas 24h a partir de `2026-10-09T11:07:16Z`.

Desbloqueio: cada chip lista os decisions daquele harness na janela de 24h, com id completo e `final_reasoning_effort` quando o campo existe e não é `unknown`. O bloco agents vazio não pode dizer que o harness não teve eventos quando os switches desse harness estão na tela.

## Abas

| Aba | O que a tela mostrou | Julgamento |
|---|---|---|
| all | chips completos; 5 min vazio; 6 switches `grok-4.7-xhigh`; agents `no active agent spawned`; `75 events`, `2↓`, `1↑`; economia `$0.00` / 2 downshifts | stats batem com a API (`total` 75, `down` 2, `up` 1, `est_usd` 0.0025) |
| antigravity | `no antigravity events in the last 24h`, `0 events` | vazio verdadeiro nesta captura: 0 eventos no log dentro de 24h |
| codex | `no codex events in the last 24h`, `0 events` | BLOCK — o log tem 3 |
| cursor | 6 linhas `grok-4.7-xhigh`; agents `no cursor events in the last 24h`; `8 events`, `4↓`, `0↑`; economia `$0.00` / 4 downshifts | BLOCK — log tem 43; o bloco agents nega a lista |
| claude-code | `no claude-code events in the last 24h`, `0 events` | BLOCK — o log tem 29 |
| kirocrew | `no kirocrew events in the last 24h`, `0 events`, 5 min vazio | vazio verdadeiro; último evento `2026-10-06T11:57:24Z` |
| grok | `no grok events in the last 24h`, `0 events` | vazio verdadeiro; 0 eventos do harness grok na janela |

## Blocos

- **Chips:** `all`, `antigravity`, `codex`, `cursor`, `claude-code`, `kirocrew`, `grok` em todas as telas.
- **Events / downshifts / upshifts:** a aba all mostra 75, 2↓ e 1↑, iguais a `stats`. As abas dos harnesses usam o rabo de 8 linhas.
- **Switches:** as linhas visíveis trazem `grok-4.7-xhigh`. O effort dessas linhas é `unknown`, então a tela não acrescenta effort. O `medium` ao lado da seta é complexity.
- **Active agents (5 min):** `none in the last 5 min`. Válido. O spawn mais novo é `2026-10-09T10:57:22Z`, cerca de 10 min antes da captura.
- **Economia estimada:** cartão separado da compactação. Na aba all, `$0.00` e 2 downshifts. A API traz `est_usd` 0.0025; a tela arredonda para duas casas.
- **Compactação:** o log está vazio (arquivo ausente). A tela mostra `0 bytes in · 0 bytes after · 0 bytes reduced · savings_pct 0.0`, `token_state unavailable`, sem nomes de harness, e a frase `Bytes medidos na saída da ferramenta. Tokens indisponíveis. Não soma na economia estimada do roteamento.`
- **Agents:** o JSON de `/api/status` é `[]`, array, nunca `null`.

## Risco residual

`by_harness_state` veio `observed` com `by_harness` vazio porque o arquivo não existe. A tela não inventou linha de harness.
