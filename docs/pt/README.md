# Downshift

Você pede um subagente. Sem o Downshift, ele herda o modelo da sessão, em geral o mais caro. Com o Downshift, a tarefa pequena vai para o modelo barato e a tarefa pesada fica no modelo forte. A escolha acontece antes do subagente começar. Você não precisa escolher o modelo na hora.

Isso não é um segundo chat e não escreve o código por você. É um roteador: lê o pedido, decide o tamanho certo e, se a troca for segura, troca.

Esta pasta explica o produto e, nos arquivos seguintes, o fluxo com o suficiente de código para produto e engenharia lerem juntos. A instalação continua no README da raiz. O mapa de saída do beta está em `docs/BETA-EXIT.md`.

## O que você ganha

Menos token no trabalho mecânico. O modelo caro só entra quando a tarefa pede desenho, risco ou muita incerteza. A decisão fica na sua máquina: sem API, sem chave, sem o texto da tarefa saindo no log.

O que você não ganha ainda: uma fatura em dólar só porque o hook rodou. Dólar de verdade só aparece quando o harness manda a quantidade de tokens depois da tarefa. E `rewrite_emitted` no log significa “o Downshift pediu a troca”, não “o harness obedeceu”.

## Como isso se parece no dia a dia

| Pedido | O que o Downshift tenta fazer |
|---|---|
| Renomear uma variável | Ficar no modelo barato |
| Um cubo 3D pequeno em um arquivo | Ficar no modelo barato |
| Uma feature com vários arquivos | Subir para o modelo do meio |
| Reescrever autenticação multi-tenant | Subir para o modelo forte |
| Um pedido vago, sem pista | Não trocar. O modelo da sessão permanece |

No Codex, barato é o Luna, o meio é o Terra e o forte é o Sol. No Claude Code, a mesma ideia vale com Haiku, Sonnet e Opus. A regra não muda de ferramenta. Mudam os nomes.

## O que cada arquivo cobre

O índice abaixo é o caminho do pedido. Cada um mistura o que o produto precisa saber e o ponto do código em que isso vive.

| Arquivo | Para quem lê |
|---|---|
| [01-entrada](01-entrada.md) | O que o Downshift enxerga e o que ignora |
| [02-classificador](02-classificador.md) | Como ele julga se a tarefa é pequena ou pesada |
| [03-marcha](03-marcha.md) | Quando sobe, quando desce, quando não mexe |
| [04-fallbacks](04-fallbacks.md) | O que acontece quando algo falha |
| [05-catalogo-e-sessao](05-catalogo-e-sessao.md) | Preço de lista versus o que a sua conta abre |
| [06-harnesses](06-harnesses.md) | Claude, Codex, Cursor e os outros |
| [07-minilm](07-minilm.md) | A decisão local, sem mandar o texto para fora |
| [08-telemetria](08-telemetria.md) | O que o relatório mostra e o que nunca grava |
| [09-decisoes](09-decisoes.md) | Por que foi feito assim |

Para ver a decisão sem abrir um harness: `downshift try "renomeie a variável" codex gpt-6-luna`. A linha `Rewrite` diz se o hook trocaria o modelo.
