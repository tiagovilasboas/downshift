# 08 · Telemetria e Observabilidade Local

O subsistema de telemetria do Downshift foi projetado para registrar o **comportamento da rota**, nunca o conteúdo do trabalho executado.

---

## Perspectiva de Produto

* **Armazenamento Seguro:** Todos os eventos são gravados exclusivamente no diretório local do usuário em `~/.downshift/events.jsonl` (herdado de `~/.harness-downshift/` se existir), com arquivo `0600` e diretório `0700`. Os appends não fazem fsync: uma queda de energia pode perder os últimos eventos, uma queda do processo não.
* **Compartilhamento Seguro de Métricas:** O comando `downshift stats --export` gera um resumo consolidado das métricas de economia e distribuição de tiers sem expor nenhum caminho de arquivo, nome de projeto ou texto de prompt.
* **Métricas em tokens:** Os tokens que o harness informa no pós-execução (`SubagentStop`, no Claude Code) são contados por classe e vinculados à decisão. As visões ao vivo (`stats`, `session`, `dsmon` e o dashboard) mostram tokens economizados em relação ao modelo pedido (`telemetry.TokenSavings`, trabalho e cache separados). `downshift report` compara tokens por tarefa concluída entre o grupo roteado e o grupo de controle. Nenhuma dessas saídas mostra dólar; `downshift session --why` diz só se os tokens de uma decisão rodariam num modelo de tier mais baixo ou mais alto que o pedido. O produto não usa preço.
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
| `input_tokens` / `output_tokens` | Métricas reais de consumo populadas caso o `PostToolUse` seja capturado |

> **Garantia de Privacidade:** Os campos `prompt`, `task`, nomes de repositórios e parâmetros de ferramentas são explicitamente omitidos do struct de serialização do evento.

---

## Compensações Técnicas (Trade-offs)

* **Vantagem:** O arquivo de telemetria pode ser compartilhado em relatórios de auditoria, issues do GitHub ou pull requests sem qualquer risco de vazamento de propriedade intelectual ou credenciais.
* **Custo:** Como o texto do prompt não é persistido, não é possível reclassificar retroativamente uma tarefa do passado a partir do arquivo de log; a depuração precisa ser realizada inspecionando diretamente as sessões dos harnesses ou usando `downshift session --why`.

O arquivo `~/.downshift/events.jsonl` é a telemetria de tokens e de rota; os backups rotacionados (100 MB por padrão) não entram nos totais do `stats`, que avisa disso.

`policy_floor` conta as decisões em que um piso de tier definido pelo operador (`.downshift/policy.json`, veja [config.md](../guides/config.md)) elevou o tier. É registrado pelo executor compartilhado para todos os harnesses.

`child_ended` registra como terminou um subagente do Claude Code (`interrupted`, `error`, `completed`, `launched`, `stopped` ou `unknown`). Não guarda o texto do erro e não conta como decisão. O `downshift stats` mostra a seção "How children ended" quando há esses eventos, e o export traz os totais em `child_status`. Quando um filho que falhou (`error` ou `interrupted`) é seguido de outro spawn na mesma sessão em até 10 minutos, a seção mostra "Respawned after a failure". É só uma pista: o spawn seguinte pode ser outro trabalho, então nunca vira rótulo de tier. Para registrar, o comando `claude-code-post-tool-use` precisa estar também no evento `PostToolUseFailure`. O campo `baseline_source` é sempre `requested` hoje. Detalhes em [stats-export.md](../reference/stats-export.md).

O `stats` agrupa os tokens observados por sessão e por subagente quando o registro de uso traz o id (hash). Não é fatura e só relata: um hook não consegue impor orçamento nem limite de concorrência. A exportação traz apenas contagens e máximos, nunca os ids.
