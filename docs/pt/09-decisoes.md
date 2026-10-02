# Decisões técnicas

| Decisão | Por quê |
|---|---|
| Go, um binário, sem módulo externo no `go.mod` | o hook tem que subir rápido e não depender de rede |
| Regex primeiro | auditável, estável, sem GPU |
| Empate vai para a classe mais alta | errar para cima numa classe era mais seguro do que deixar um `rearchitect` no modelo barato |
| `SIMPLE` no tier small | uma mudança isolada não justifica o modelo do meio; o pulo Luna → Terra era conservador demais |
| Upshift e downshift exigem confiança | sem margem, o modelo atual fica. Antes só o downshift tinha esse freio, e o upshift encarecia fácil |
| MiniLM não é o padrão neural | o hash cabe no binário; o neural não melhorou o tier no holdout o bastante para pagar Python |
| Jev fica de fora do caminho | decisão remota quebra privacidade e o custo zero da triagem |
| Catálogo não é entitlement | a conta recusar `gpt-6-luna` é problema do Codex, não do roteador |
| `rewrite_emitted` não é ACK | o harness pode ignorar o modelo pedido |
| Prompt não entra no log | o evento prova a rota, não o conteúdo do trabalho |

O que ainda não está provado: uma semana de `usage` com dólar de provedor, e o Claude Code honrando o rewrite numa conta com a ferramenta Task. O Codex já mostrou, numa sessão local, o spawn seguinte chegar no modelo que o hook tinha emitido.
