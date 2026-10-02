# Marcha

A classe vira tier em `Complexity.Tier()`, igual em todo harness:

| Classe | Tier | Ideia |
|---|---|---|
| TRIVIAL | small | trabalho mecânico |
| SIMPLE | small | uma mudança isolada, continua no modelo barato |
| MEDIUM | mid | feature de verdade |
| COMPLEX | frontier | desenho de sistema, risco alto |

Antes, `SIMPLE` também ia para `mid`. Um campo novo ou um cubo three.js saía do Luna e ia para o Terra, ~25 vezes mais caro na entrada do catálogo Codex. Isso era conservador demais. `SIMPLE` agora fica em `small`. `MEDIUM` continua subindo.

O veredito compara o tier do modelo atual com o tier pedido:

- atual acima do pedido: `DOWNSHIFT`
- igual: `OK`
- atual abaixo: `UPSHIFT`
- modelo atual desconhecido: `UNKNOWN`

A troca só acontece se `ShouldRewriteModel` for verdadeiro.

- `OK` não troca.
- `DOWNSHIFT` ou `UPSHIFT` sem confiança não troca. O veredito fica no log. O modelo atual permanece.
- Com confiança, a troca segue.
- Modelo marcado `explicit_only` no catálogo não é substituído.

No Codex, com o catálogo atual: small é `gpt-6-luna`, mid é `gpt-5.6-terra`, frontier é `gpt-6-sol`. Um cubo three.js (`SIMPLE`) no Luna fica no Luna. Um export CSV (`MEDIUM`) no Luna sobe para o Terra. Um `rearchitect` sobe para o Sol.
