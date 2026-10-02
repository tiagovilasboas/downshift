# Classificador

O classificador responde uma pergunta de produto: esta tarefa é mecânica, isolada, uma feature ou um desenho de sistema?

**Produto.**

| Classe | Em português | Exemplo |
|---|---|---|
| TRIVIAL | trabalho mecânico | rename, typo, git status |
| SIMPLE | uma mudança só | um campo, um cubo three.js, um arquivo |
| MEDIUM | uma feature | export CSV, um fluxo com vários arquivos |
| COMPLEX | desenho de sistema | rearchitect, service mesh, etcd, raft |

Se nada disso bater, a tarefa vira `MEDIUM` e o Downshift desconfia. Desconfiado, ele prefere não mexer no modelo.

**Técnico.** Os votos estão em `internal/core/signals.go`. Cada regex tem um peso. A classe vencedora é a de maior soma. Empate sobe. Confiança exige pelo menos 2 pontos de diferença para a segunda. `implement` pesa 2 em `MEDIUM`. `three.js scene` ou `rotating cube` pesam 3 em `SIMPLE` e vencem o `implement`. `service mesh` e `raft` pesam 3 em `COMPLEX` e vencem o `implement`.

O Graphify pode subir para `COMPLEX` se o texto citar arquivo ou símbolo. No caminho padrão o fetcher é nulo: só vale o que está escrito. O boost semântico (`internal/semantic`) só sobe a classe. `DOWNSHIFT_MINILM=0` desliga.

**Trade-off.** Regex é auditável e não pede GPU. Não entende intenção além das palavras. Por isso o cubo e o service mesh precisam de sinais explícitos, em vez de um modelo de linguagem no meio do hook.
