# Fallbacks

Se algo der errado, o subagente ainda nasce. O Downshift não segura o trabalho para “pensar melhor”.

**Produto.** Na dúvida, o modelo da sessão permanece. Você não perde o spawn por causa do roteador. O que você perde, nesses casos, é a economia ou a proteção de um modelo mais forte.

**Técnico.**

| Falha | Resultado |
|---|---|
| JSON inválido | `INVALID_EVENT`, spawn segue |
| stdin > 1 MB | `PAYLOAD_TOO_LARGE`, sem classificação |
| timeout de leitura | `INPUT_TIMEOUT` |
| sessão sem lista de modelos | não reescreve |
| id recomendado fora da sessão | não escreve id inventado |
| upshift ou downshift sem confiança | não reescreve |
| embedder externo falha | hash local |
| hash ou protótipo falha | fica o regex |
| `DOWNSHIFT_NO_ROUTE=1` | classifica, grava `baseline`, não troca |

Nenhum desses caminhos chama um LLM para decidir.

**Trade-off.** Fail-open protege o fluxo. Um payload enorme no Codex simplesmente não é roteado, em vez de derrubar a sessão. O preço é um spawn no modelo da sessão, que pode ser o caro.
