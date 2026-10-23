# Itens com dependência externa

**O downshift não fecha esses itens sozinho.** Cada um tem um bloqueio fora do repo: outro app, rate limit, dados de uso real, ou sessão ao vivo de outro harness.

Ficam aqui até o bloqueio ser removido, depois migram para uma fase ativa.

---

## Precisa de sessão ao vivo em outro harness

| Item | Pts | Bloqueio | O que falta |
|------|-----|----------|------------|
| A-P5 | 3 | Cursor ou Codex rodando com um spawn não-cifrado | O código dos adapters está completo (`updated_input` emitido). Falta capturar um payload ao vivo para confirmar que `cursorTaskModel` aceita o slug atual e que o Codex recebe o JSON correto. Ação: abrir Cursor, spawnar um subagente, rodar `downshift stats --days=1`. |

## Precisa de rate limit ou deploy externo

| Item | Pts | Bloqueio | O que falta |
|------|-----|----------|------------|
| E-M5 | 3 | Rate limit Vercel (100 deploys/dia, bloqueou em 23/10) | SEO polish: metadata dinâmica por página, structured data, Lighthouse ≥ 90 |
| E-M6 | 4 | Rate limit Vercel | Deploy e validação: `vercel.json` multi-app, CSP hashes, Lighthouse em prod |

## Precisa de dados de uso real

| Item | Pts | Bloqueio | O que falta |
|------|-----|----------|------------|
| C-3 | 3 | Volume real das famílias `bazel`, `nx`, `psql` via `downshift learning` | Só implementar se aparecerem no uso. Sem dados, é YAGNI. |
| C-5 | 2 | Mais amostras de `go test -v` com `t.Log`, `-parallel`, multi-pacote | Hoje só 1 amostra. Medir antes de abrir novos parsers. |
| A-P4 | 2 | Payload de child-end capturado ao vivo | Parsear tokens do child-end num `postToolUse` real. |

## Coleta passiva — acontece com uso, não com código

| Item | O que é |
|------|---------|
| D-4 | Child-token payloads de subagentes |
| D-5 | Control group com 20 tarefas concluídas para o `report` |
| R-1/R-2 | 20+ controles `NO_ROUTE` + 20+ tarefas independentes para `downshift report` publicável |

## Integrações externas — fora do controle do repo

| Item | Pts | Por que não é responsabilidade do downshift |
|------|-----|---------------------------------------------|
| D-1 | 5 | OpenRouter shadow routing: depende da API e contratos do OpenRouter |
| D-2 | 5 | OpenCode wiring: depende da integração OpenCode + OpenRouter estar estável |

---

## Como usar este arquivo

Quando um bloqueio for removido (rate limit expirou, payload capturado, dados de uso disponíveis), mover o item para `phase6-downshift-owned.md` ou abrir uma nova fase.

Não criar código para estes itens enquanto o bloqueio existir.
