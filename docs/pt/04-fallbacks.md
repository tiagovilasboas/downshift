# Fallbacks

A regra de ouro é não impedir o spawn.

| Falha | O que acontece |
|---|---|
| JSON inválido | `INVALID_EVENT`, o harness segue com o modelo que já ia usar |
| stdin estourou 1 MB | `PAYLOAD_TOO_LARGE`, sem classificação |
| timeout de leitura | `INPUT_TIMEOUT` |
| sessão sem lista de modelos | não reescreve; o modelo atual fica |
| id recomendado fora da sessão | não escreve um id que a sessão não tem |
| upshift ou downshift sem confiança | não reescreve |
| embedder externo falha | volta para o hash local |
| hash ou protótipo falha | fica a classe do regex |
| `DOWNSHIFT_NO_ROUTE=1` | classifica e grava `baseline`, mas não troca o modelo |

Nenhum desses caminhos chama um LLM para decidir. O único modelo generativo é o que o harness vai usar depois, no subagente.
