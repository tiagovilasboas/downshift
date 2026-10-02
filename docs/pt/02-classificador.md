# Classificador

O classificador é determinístico. Está em `internal/core/signals.go` e `internal/core/classifier.go`. Não chama rede.

Cada sinal é uma regex, um peso e uma classe: `TRIVIAL`, `SIMPLE`, `MEDIUM` ou `COMPLEX`. O texto é passado para minúsculas. Os pesos da mesma classe somam. Empate fica com a classe mais alta. Sem nenhum sinal, o resultado é `MEDIUM` sem confiança.

Confiança existe quando a classe vencedora ganha da segunda por pelo menos 2 pontos.

Exemplos que o código trata hoje:

| Texto | Classe | Por quê |
|---|---|---|
| rename, typo, git status | TRIVIAL | sinal mecânico, peso 3 |
| add a field, write a function, rotating cube, three.js scene | SIMPLE | uma mudança isolada |
| implement the feature, debug sem sinal mais forte | MEDIUM | `implement` e `feature` pesam 2 cada |
| rearchitect, service mesh, etcd, raft, multi-tenant | COMPLEX | sinal de sistema, peso 3, ganha de `implement` |

`implement` sozinho não é complexo. `implement` mais `service mesh` ou `raft` é complexo, porque esses sinais pesam mais.

O Graphify (`internal/graphify`) pode subir uma classe que ainda não é `COMPLEX` quando o texto cita arquivo ou símbolo de um grafo. No hook de produção o fetcher é nulo: só vale o que está escrito no prompt. Não há chamada MCP no caminho padrão.

O boost semântico (`internal/semantic`) pode subir a classe, nunca descer. Está ligado por padrão. `DOWNSHIFT_MINILM=0` desliga.
