# Telemetria

Tudo fica em `~/.harness-downshift/`, com permissão só do usuário. Não vai para o git.

`events.jsonl` guarda complexidade, tier, veredito, modelos pedido e final, harness, esforço, outcome e um hash de sessão. Não guarda prompt, path nem o id cru da sessão.

Outcomes que importam:

| Outcome | Significado |
|---|---|
| `rewrite_emitted` | o hook emitiu uma decisão de modelo |
| `baseline` | `DOWNSHIFT_NO_ROUTE=1`, classificou e não trocou |
| `usage` | tokens reais ligados a uma decisão anterior |
| `error` | `PAYLOAD_TOO_LARGE`, JSON inválido, timeout |

`downshift stats --export` resume isso sem o texto das tarefas. Custo em dólar só aparece quando existe `usage` com tokens. Até lá o número é estimativa de lista, marcada como estimativa.
