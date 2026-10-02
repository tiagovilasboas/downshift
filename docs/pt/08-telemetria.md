# Telemetria

O relatório existe para você ver a rota, não o trabalho.

**Produto.** Os eventos ficam na sua máquina, em `~/.harness-downshift/`. Dá para exportar um resumo sem o texto das tarefas. Economia em dólar só aparece quando o harness manda tokens. Até lá o número é estimativa de tabela, marcada como estimativa.

**Técnico.** `events.jsonl` guarda complexidade, tier, veredito, modelos pedido e final, harness, esforço, outcome e hash de sessão. Não guarda prompt, path nem o id cru. `rewrite_emitted` é emissão. `baseline` é `DOWNSHIFT_NO_ROUTE=1`. `usage` são tokens ligados a uma decisão anterior. `error` cobre `PAYLOAD_TOO_LARGE`, JSON inválido e timeout. `downshift stats --export` resume sem o texto.

**Trade-off.** Sem prompt no log, o arquivo pode ir para um issue. Sem prompt, também não dá para reler a tarefa original a partir do JSONL. A prova de conteúdo continua sendo a sessão do harness, não o Downshift.
