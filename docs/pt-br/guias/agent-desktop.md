# agent-desktop: leituras do desktop compactadas

> Downshift por Tiago Vilas Boas  
> Repositório oficial: https://github.com/tiagovilasboas/downshift-ai  
> Licença: Apache 2.0

O `agent-desktop` é uma ferramenta de linha de comando que deixa um agente de código controlar apps do desktop
no macOS, Windows e Linux. Em vez de tirar print, ele lê a árvore de acessibilidade do app e a imprime em JSON,
com um ref (`@s8f3k2p9:e12`) para cada elemento, e age sobre esses refs. Depois de cada ação o agente lê a tela
de novo para conferir o resultado, então a mesma árvore volta muitas vezes.

O Downshift não instala o `agent-desktop` e não controla o desktop. Ele encurta o que o `agent-desktop`
imprime, em dois passos:

1. **Visão compacta.** Um `snapshot` ou `find` vira uma linha por elemento. Nenhum campo se perde: um teste
   decodifica a visão de volta para o mesmo JSON.
2. **Delta.** Uma releitura da mesma janela no mesmo escopo vai só com as linhas que mudaram.

## Números

Medidos com o binário `downshift` sobre fixtures sintéticos, montados no formato de saída do `agent-desktop`,
e não num desktop real. Meça no seu desktop antes de citar.

| Leitura | JSON bruto | Impresso | Economia |
| --- | --: | --: | --: |
| Gerenciador de arquivos, primeira leitura (219 refs) | 30.272 B | 14.183 B | 53,1% |
| App de chat, primeira leitura (175 refs) | 26.861 B | 10.802 B | 59,8% |
| `find`, 18 botões | 3.899 B | 1.455 B | 62,7% |
| Mesma janela de chat depois de uma ação (2 linhas mudaram) | 26.839 B | 764 B | 97,2% |
| Mesma janela de chat, sem mudança | 26.839 B | 567 B | 97,9% |

Nas três leituras do chat (ler, agir, conferir duas vezes), 80.539 bytes viram 12.133: 84,9% a menos. As flags
`-i`, `--compact` e `--skeleton` do próprio `agent-desktop` continuam valendo e somam com isso.

Cada leitura é uma execução normal do Compactor: entra no `exec-events.jsonl` como família `agent-desktop` e
aparece no `downshift report`, no `downshift stats`, no painel de sessão e no pane do `downshift-agents`. Bytes
lidos de volta com `downshift retrieve` são descontados, como em qualquer família.

## Como ativa

| Onde | O que fazer |
| --- | --- |
| Claude Code com o hook do Downshift | Nada. `agent-desktop snapshot ...` e `agent-desktop find ...` são reescritos para `downshift compact` com a sessão. |
| Qualquer outro harness, ou um script | Rode `downshift compact -- agent-desktop snapshot --app Files -i`. Os dois passos valem; o delta usa o diretório de trabalho como escopo. |
| `agent-desktop mcp` (ferramentas MCP) | Não compacta: o Downshift não vê o resultado de ferramentas MCP. Use a forma de linha de comando. |

Só `snapshot` e `find` são reescritos, com o subcomando primeiro. Ações (`click`, `type`, ...) nunca. Uma
leitura que falhou (`"ok": false`, saída 1) passa byte a byte.

## O delta

O `agent-desktop` numera os refs em profundidade sobre os elementos acionáveis. Com a árvore igual, os números
são os mesmos e só o id da leitura muda; refs de leituras anteriores continuam válidos. Então uma linha igual
nas duas visões é o mesmo elemento com o mesmo ref.

O delta só vai quando tudo isto vale; senão, a visão inteira vai:

- Mesma linha de comando, no mesmo escopo (sessão do hook ou diretório de trabalho), em até 10 minutos.
- As duas leituras completas (nada de `complete=false`, `truncated=true` ou `identity_unproven=true`).
- A leitura bruta foi guardada pelo [CCR](compactacao-e-ccr.md), então `downshift retrieve <id>` sempre traz tudo.
- O delta tem menos de 60% da visão.

A visão anterior fica em `~/.downshift/desktopview/`, um arquivo privado (0600) por janela e escopo, com linhas
que parecem segredo retidas. `DOWNSHIFT_DESKTOP_DELTA=0` manda toda leitura inteira.

## Limites

- **Escopo compartilhado.** Subagentes da mesma sessão, ou chamadas diretas do mesmo diretório, dividem o
  escopo: um agente pode receber um delta de uma leitura que não viu. O delta avisa e aponta o `retrieve`. Se
  isso acontecer muito, use `DOWNSHIFT_DESKTOP_DELTA=0`.
- **Segredos.** O `agent-desktop` imprime tudo numa linha. Se algum campo parecer segredo, o CCR retém a
  leitura bruta inteira e o `retrieve` não traz a árvore; a visão compacta e o delta não mudam.
- **Medição.** Os números vêm de fixtures sintéticos. Falta medir num desktop real: rode a mesma tarefa com
  `DOWNSHIFT_EXEC=0` e com o hook ligado e compare o `downshift report`.

Guia completo em inglês: [agent-desktop](../../guides/agent-desktop.md).
