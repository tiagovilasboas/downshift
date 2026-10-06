# 08 · Telemetria e Observabilidade Local

O subsistema de telemetria do Downshift foi projetado para registrar o **comportamento da rota**, nunca o conteúdo do trabalho executado.

---

## Perspectiva de Produto

* **Armazenamento Seguro:** Todos os eventos são gravados exclusivamente no diretório local do usuário em `~/.harness-downshift/events.jsonl` com permissões restritas (`0700`).
* **Compartilhamento Seguro de Métricas:** O comando `downshift stats --export` gera um resumo consolidado das métricas de economia e distribuição de tiers sem expor nenhum caminho de arquivo, nome de projeto ou texto de prompt.
* **Métricas em Dólares Reais:** Cálculos baseados em dólares reais (`Real provider cost`) são calculados quando o harness fornece o consumo real de tokens no pós-execução (`PostToolUse`). Na ausência desses dados, o sistema apresenta a economia normalizada em unidades adimensionais baseadas nos preços de tabela dos modelos.
* **Honor inferido:** `rewrite_honored_inferred` no export conta quando o spawn seguinte da mesma sessão chega no modelo que o hook tinha pedido (ou quando `rewrite_honored` veio preenchido). O JSONL original não é alterado.

---

## Detalhes de Engenharia

Cada entrada registrada no arquivo `events.jsonl` obedece a um esquema estrito:

| Campo | Descrição e Papel |
|---|---|
| `correlation_id` | Identificador único opaco gerado para cada chamada do hook |
| `timestamp` | Horário UTC no formato RFC3339Nano |
| `complexity` | Complexidade identificada (`TRIVIAL`, `SIMPLE`, `MEDIUM`, `COMPLEX`) |
| `requested_model` | Modelo de origem informado pelo cliente (ou `unknown` se omitido) |
| `final_model` | Modelo final atribuído para a execução do subagente |
| `verdict` | Decisão de roteamento (`DOWNSHIFT`, `OK`, `UPSHIFT` ou `UNKNOWN`) |
| `tier` | Tier selecionado (`small`, `mid`, `frontier`) |
| `outcome` | Estado do ciclo: `rewrite_emitted`, `baseline`, `usage` ou `error` |
| `session_id` | Hash criptográfico anônimo da sessão (o ID bruto nunca é persistido) |
| `input_tokens` / `actual_cost_usd` | Métricas reais de consumo populadas caso o `PostToolUse` seja capturado |

> **Garantia de Privacidade:** Os campos `prompt`, `task`, nomes de repositórios e parâmetros de ferramentas são explicitamente omitidos do struct de serialização do evento.

---

## Compensações Técnicas (Trade-offs)

* **Vantagem:** O arquivo de telemetria pode ser compartilhado em relatórios de auditoria, issues do GitHub ou pull requests sem qualquer risco de vazamento de propriedade intelectual ou credenciais.
* **Custo:** Como o texto do prompt não é persistido, não é possível reclassificar retroativamente uma tarefa do passado a partir do arquivo de log; a depuração precisa ser realizada inspecionando diretamente as sessões dos harnesses ou usando o comando `downshift try`.

O arquivo `~/.harness-downshift/events.jsonl` é a telemetria de custo e de rota. O loop de revisão fica em `~/.harness-downshift/loop-events.jsonl`. Com shadow ligado, esse segundo arquivo ganha `classifier_shadow` (números e hash do artefato, sem texto da tarefa). `downshift feedback <id> success` não cria rótulo de tier mínimo. O guia está em [10 · Candidato em shadow](10-candidato-shadow.md).
