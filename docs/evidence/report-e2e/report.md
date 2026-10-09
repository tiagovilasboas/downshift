# Report e2e — 2026-10-09T06:54:33Z

Captura headless do Chrome em `http://127.0.0.1:7474/?e2e=…` (cache desligado, `app.js?v=12`). Um PNG por chip. O texto abaixo é o que a página mostrava nesse instante.

## all.png

Aba **all** com dado, não só título.

- **Active agents / spawned in the last 5 min:** 7 linhas. A de 2 min é só `cursor` / `grok-4.7-high`. As de 4 min ainda são os pares gravados antes da troca do binário.
- **Switch:** id completo `grok-4.7-high` (não `grok`). Onde a API traz effort de verdade, a linha mostra `muse-spark-1.3-high · high` e `claude-opus-4-8 · medium`. O `↓ medium` ao lado do grok é a complexidade `MEDIUM`; nesses eventos `final_reasoning_effort` é `unknown`.
- **Stats:** 108 events, 1↓, 9↑.
- **Estimated savings:** `$0.00`, 1 downshift aplicado nas últimas 24h. Cartão separado da compactação.
- **Session compaction:** `0 bytes in · 0 bytes after · 0 bytes reduced · savings_pct 0.0`, `token_state unavailable`, e a frase “Bytes medidos na saída da ferramenta. Tokens indisponíveis. Não soma na economia estimada do roteamento.” Não há `context-sensor.json`; os zeros são a leitura vazia, não uma economia inventada.

## antigravity.png

Chip antigravity. Uma linha, `flash · medium`, stats `1 events · antigravity only`. Nada de cursor nem de claude-code. Janela de 5 min vazia.

## codex.png

Chip codex. Três linhas do harness: `gpt-5.5 · medium`, `unknown`, `gpt-5.6-terra`. Stats `3 events · codex only`. Janela de 5 min vazia.

## cursor.png

Chip cursor. As linhas de 5 min dizem `cursor`. O switch traz `grok-4.7-high` e `claude-opus-5-thinking-high · medium`. Não aparece `claude-opus-4-8`. Stats `50 events · cursor only`.

## claude-code.png

Chip claude-code. As 3 linhas de 5 min dizem `claude-code`. O switch traz `grok-4.7-high` e `claude-opus-4-8 · medium`. Não aparece `claude-opus-5-thinking-high` nem `muse-spark-1.3-high`. A linha nova de 2 min (só cursor) não entrou nesta aba. Stats `54 events · claude-code only`.

## kirocrew.png

O chip existe. Estado vazio explícito: “no kirocrew switches in the last 24h”, “none in the last 5 min”, stats `0 events · kirocrew only`. O último evento real no log é `2026-10-06T11:57:24Z`, fora da janela. Nenhuma linha foi inventada.
