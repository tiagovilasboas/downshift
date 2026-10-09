# Report e2e — PASS

Captura `2026-10-09T12:12:50.729Z` em `http://127.0.0.1:7474`. Health `200` às `2026-10-09T12:12:20Z` (`{"ok":true}`). O listener já devolvia a janela inteira (76 switches) e foi mantido: PID 28355, `./downshift serve`, iniciado `2026-10-09 08:58:49` local (11:58:49Z). Binário `./downshift` com mtime local `2026-10-09 08:10:51`.

`go test ./internal/core/ ./internal/server/ ./internal/benchmark/ ./cmd/downshift/` — os quatro pacotes `ok` (cache).

Sensor: `node --experimental-websocket docs/evidence/report-e2e/live-report.check.mjs` (exit 0). `findings.json`: `verdict` PASS, `findings` vazio. PNGs: `all.png`, `antigravity.png`, `codex.png`, `cursor.png`, `claude-code.png`, `kirocrew.png`, `grok.png`. Relógio da tela: 9:12:49 AM local.

Log de 24h (`~/.harness-downshift/events.jsonl`, corte na captura): codex 2, cursor 45, claude-code 29. Antigravity, kirocrew e grok: 0. `/api/status` devolve os mesmos 76 switches (`agents` é `[]`). Compactação: `~/.harness-downshift/context-compactions.jsonl` não existe. `compaction.by_harness` veio `{}`. Nenhum nome de harness foi inventado.

## Abas

| Aba | O que a tela mostrou | Julgamento |
|---|---|---|
| all | chips completos; 2 ativos nos últimos 5 min (`grok-4.7-xhigh`, cursor, 42s e 48s); 6 switches visíveis, os mais novos; agents `no spawned agents in the agent log`; `76 events`, `2↓`, `1↑`; economia `$0.00` / 2 downshifts | conta bate com `stats` (`total` 76, `down` 2, `up` 1, `est_usd` 0.0025, arredondado para `$0.00`) e com o log |
| antigravity | `no antigravity events in the last 24h`, `0 events`, 5 min vazio | vazio verdadeiro: 0 eventos no log |
| codex | `2 events`, `0↓`, `1↑`; linhas `gpt-5.5 · medium` (02:57 PM, `2026-10-08T17:57:51Z`, UPSHIFT) e `unknown` (01:04 PM, `2026-10-08T16:04:24Z`); 5 min vazio | bate com o log (2). Não está vazio |
| cursor | `45 events`, `22↓`, `0↑`; economia `$0.00` / 22 downshifts; 2 ativos (`grok-4.7-xhigh`); agents não nega eventos do cursor | bate com o log (45). Não é o rabo de 8 linhas |
| claude-code | `29 events`, `14↓`, `0↑`; economia `$0.01` / 14 downshifts; a lista visível inclui `claude-opus-4-8 · medium` (`2026-10-09T05:29:11Z`); 5 min vazio | bate com o log (29). Não está vazio |
| kirocrew | `no kirocrew events in the last 24h`, `0 events`, 5 min vazio | vazio explícito. Último evento no log: `2026-10-06T11:57:24.855122Z`, fora da janela |
| grok | `no grok events in the last 24h`, `0 events`, 5 min vazio | vazio verdadeiro: 0 eventos do harness grok na janela |

## Blocos

- **Chips:** `all`, `antigravity`, `codex`, `cursor`, `claude-code`, `kirocrew`, `grok` em todas as telas. O chip da aba fica selecionado.
- **Events / downshifts / upshifts:** all usa `stats` (76, 2↓, 1↑). Cada aba de harness conta o próprio recorte de 24h: codex 2/0↓/1↑, cursor 45/22↓/0↑, claude-code 29/14↓/0↑, antigravity, kirocrew e grok em 0.
- **Switches:** a lista pintada continua sendo as 6 linhas mais novas do recorte. A contagem é a janela. No cursor, essas 6 são `grok-4.7-xhigh` com `final_reasoning_effort=unknown` (`2026-10-09T10:25:23Z` até `2026-10-09T12:12:06Z`), então a tela não acrescenta effort nelas. O `medium` ao lado da seta é complexity. Codex mostra `medium` em `gpt-5.5`. Claude Code mostra `medium` em `claude-opus-4-8`.
- **Active agents (5 min):** all e cursor listam os dois spawns `2026-10-09T12:12:00Z` e `2026-10-09T12:12:06Z`. As outras abas mostram `none in the last 5 min`.
- **Economia estimada:** cartão separado da compactação. All `$0.00` / 2 downshifts. Cursor `$0.00` / 22 downshifts (`estimated_savings` somado no recorte = 0.0025). Claude Code `$0.01` / 14 downshifts (0.00925). Abas sem DOWNSHIFT não mostram o cartão.
- **Compactação:** arquivo ausente. A tela mostra `0 bytes in · 0 bytes after · 0 bytes reduced · savings_pct 0.0`, `token_state unavailable`, sem nomes de harness, e a frase `Bytes medidos na saída da ferramenta. Tokens indisponíveis. Não soma na economia estimada do roteamento.`
- **Agents:** o JSON de `/api/status` é `[]`. O bloco da tela diz `no spawned agents in the agent log`. Não diz que o harness da aba não teve eventos.

## Risco residual

`by_harness_state` veio `observed` com `by_harness` vazio porque o arquivo de compactação não existe. A tela não inventou linha de harness.

O `2↓` da aba all conta só DOWNSHIFT com rewrite aplicado (`stats.down`). O `22↓` do cursor e o `14↓` do claude-code contam todo verdict DOWNSHIFT do recorte. São contadores diferentes; o sensor desta captura trata isso como contrato.

A lista visível de switches é a cauda de 6 linhas. Eventos com effort no meio da janela, como `2026-10-09T00:27:58Z` `muse-spark-1.3-high` · `high` no cursor, entram na contagem 45 e não aparecem nessas 6 linhas.
