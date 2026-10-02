# Marcha

A marcha diz se o modelo atual está caro demais, barato demais ou no tamanho certo. A regra é a mesma em Claude, Codex, Cursor, Antigravity e KiroCrew. Mudam os nomes dos modelos.

**Produto.**

| Tarefa | Tamanho | O que o Downshift faz se você está no modelo barato |
|---|---|---|
| TRIVIAL | small | fica |
| SIMPLE | small | fica |
| MEDIUM | mid | sobe para o modelo do meio |
| COMPLEX | frontier | sobe para o modelo forte |

No Codex, barato é `gpt-6-luna`, o meio é `gpt-5.6-terra` e o forte é `gpt-6-sol`. Um cubo three.js fica no Luna. Um export CSV sobe para o Terra. Um `rearchitect` sobe para o Sol.

Antes, `SIMPLE` ia para o meio. Campo novo e cubo 3D saíam do Luna para o Terra, cerca de 25 vezes mais caro na entrada do catálogo. Isso era conservador demais para o produto.

**Técnico.** `Complexity.Tier()` em `internal/core/policy.go` mapeia a classe. O veredito compara o tier atual com o pedido: `DOWNSHIFT`, `OK`, `UPSHIFT` ou `UNKNOWN`. `ShouldRewriteModel` só autoriza a troca em downshift ou upshift **com confiança**. Sem os 2 pontos de margem, o veredito aparece no log e o modelo atual permanece. `explicit_only` no catálogo nunca é substituído.

**Trade-off.** O freio de confiança vale para os dois lados. Uma tarefa `MEDIUM` incerta no Luna fica no Luna, mesmo com veredito `UPSHIFT`. O produto paga menos token. Engenheiro lê a intenção no relatório. Se o Luna for fraco demais para um `SIMPLE` limítrofe, o erro é para o barato.
