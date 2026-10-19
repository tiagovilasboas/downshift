# 08 · Telemetria e Observabilidade Local

O subsistema de telemetria do Downshift foi projetado para registrar o **comportamento da rota**, nunca o conteúdo do trabalho executado.

---

## Perspectiva de Produto

* **Armazenamento Seguro:** Todos os eventos são gravados exclusivamente no diretório local do usuário em `~/.downshift/events.jsonl` (herdado de `~/.harness-downshift/` se existir), com arquivo `0600` e diretório `0700`. Os appends não fazem fsync: uma queda de energia pode perder os últimos eventos, uma queda do processo não.
* **Compartilhamento Seguro de Métricas:** O comando `downshift stats --export` gera um resumo consolidado das métricas de economia e distribuição de tiers sem expor nenhum caminho de arquivo, nome de projeto ou texto de prompt.
* **Métricas em Dólares Reais:** Cálculos baseados em dólares reais (`Real provider cost`) são calculados quando o harness fornece o consumo real de tokens no pós-execução (`PostToolUse`). Na ausência desses dados, o sistema apresenta a economia normalizada em unidades adimensionais baseadas nos preços de tabela dos modelos. As visões ao vivo (`stats`, `session`, `dsmon` e o dashboard) mostram tokens economizados em relação ao modelo pedido (`telemetry.TokenSavings`, trabalho e cache separados), nunca dólares; `downshift try` diz só se os tokens de uma decisão rodariam num modelo mais barato ou mais caro que o pedido. Dólares aparecem só em `downshift report`, precificados na hora do relatório, como estimativa a preço de lista; os preços do catálogo carregam a data `prices_as_of`.
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

O arquivo `~/.downshift/events.jsonl` é a telemetria de custo e de rota; os backups rotacionados (100 MB por padrão) não entram nos totais do `stats`, que avisa disso. O loop de revisão fica em `~/.downshift/loop-events.jsonl`. Com shadow ligado, esse segundo arquivo ganha `classifier_shadow` (números e hash do artefato, sem texto da tarefa). `downshift feedback <id> success` não cria rótulo de tier mínimo. O guia está em [10 · Candidato em shadow](10-candidato-shadow.md).

`policy_floor` conta as decisões em que um piso de tier definido pelo operador (`.downshift/policy.json`, veja [config.md](../guides/config.md)) elevou o tier. É registrado pelo executor compartilhado para todos os harnesses.

`child_ended` registra como terminou um subagente do Claude Code (`interrupted`, `error`, `completed`, `launched`, `stopped` ou `unknown`). Não guarda o texto do erro e não conta como decisão. O `downshift stats` mostra a seção "How children ended" quando há esses eventos, e o export traz os totais em `child_status`. Quando um filho que falhou (`error` ou `interrupted`) é seguido de outro spawn na mesma sessão em até 10 minutos, a seção mostra "Respawned after a failure". É só uma pista: o spawn seguinte pode ser outro trabalho, então nunca vira rótulo de tier. Para registrar, o comando `claude-code-post-tool-use` precisa estar também no evento `PostToolUseFailure`. O campo `baseline_source` é sempre `requested` hoje. Detalhes em [stats-export.md](../reference/stats-export.md).

O `stats` agrupa o custo observado, precificado pelo catálogo, por sessão e por subagente quando o registro de uso traz o id (hash). É estimativa, não fatura, e só relata: um hook não consegue impor orçamento nem limite de concorrência. A exportação traz apenas contagens e máximos, nunca os ids.
