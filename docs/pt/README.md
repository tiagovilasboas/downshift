# Downshift

O Downshift escolhe o modelo de um subagente antes dele nascer. O harness ia usar o modelo da sessão, em geral o mais caro, até para renomear uma variável. O hook olha o texto da tarefa, decide o tier e, se a troca for segura, escreve outro id.

Não é um chat. Não resume contexto, não lê o repositório e não gera a resposta. A geração continua no modelo que o harness executar. A triagem é local: um binário Go, sem rede e sem chave.

O README da raiz é o guia de instalação. Esta pasta é a arquitetura, em português, no fluxo em que o código realmente roda.

## O que esta documentação atende

Serve para quem vai alterar a marcha, o classificador ou um adapter e precisa saber o que é regra e o que é acidente. Também serve para ler um `events.jsonl` sem confundir emissão com modelo honrado.

Não substitui o quickstart, a matriz de harness nem o mapa de saída do beta (`docs/BETA-EXIT.md`). Não promete economia em dólar: isso só existe quando um evento `usage` traz tokens do provedor.

## O fluxo, com a decisão e o trade-off

**1. Entra só o spawn.** O adapter tira o texto da tarefa e o modelo atual. O resto da conversa fica de fora.

Decisão: classificar o pedido do subagente, não o fio inteiro. Trade-off: um jogo 3D descrito em uma frase curta pode parecer simples, e um payload enorme do Codex passa de 1 MB e nem é classificado (`PAYLOAD_TOO_LARGE`). Ganho: o prompt não vai para log nem para uma API.

**2. O regex vota numa classe.** `TRIVIAL` e `SIMPLE` são trabalho pequeno. `MEDIUM` é feature. `COMPLEX` é desenho de sistema. Empate sobe a classe. Sem sinal, fica `MEDIUM` sem confiança.

Decisão: sinais explícitos, com peso, em `internal/core/signals.go`. Trade-off: `implement` sozinho não distingue um cubo three.js de um export CSV. Por isso um artefato pequeno (`three.js scene`, `rotating cube`, um arquivo) pesa como `SIMPLE`, e `service mesh` ou `raft` pesam como `COMPLEX` e ganham de `implement`.

**3. O semântico só sobe.** O hash local, ligado por padrão, pode elevar a classe. Nunca desce. O MiniLM neural é opcional e cai no hash se o comando falhar.

Decisão: não colocar um classificador remoto no caminho. Trade-off: o hash erra diferente do regex e às vezes sobe um cubo para `MEDIUM` sem confiança. A troca, nesse caso, não acontece. O neural, no holdout, zerou FRONTIER→MID e não ganhou acerto de tier o bastante para virar padrão.

**4. A classe vira tier.** `TRIVIAL` e `SIMPLE` ficam em small. `MEDIUM` pede mid. `COMPLEX` pede frontier. A regra é a mesma em todo harness. Mudam os ids do catálogo.

Decisão: `SIMPLE` não divide o tier mid com `MEDIUM`. Trade-off: um campo novo ou uma cena three.js permanece no modelo barato (`gpt-6-luna` no Codex). Uma feature (`implement the feature…`) ainda sobe para o mid (`gpt-5.6-terra`). Se o small for fraco demais para um `SIMPLE` limítrofe, a gente erra para o barato, não para o caro.

**5. A confiança autoriza a troca.** Veredito `OK` não troca. Upshift e downshift sem margem de 2 pontos também não. Com confiança, troca. Modelo `explicit_only` nunca é substituído.

Decisão: o freio vale para os dois lados. Antes, a dúvida podia encarecer e não podia baratear. Trade-off: uma tarefa `MEDIUM` incerta no Luna fica no Luna, mesmo que o veredito diga `UPSHIFT`. O log mostra a intenção. O hook não reescreve.

**6. O id tem que existir na sessão.** O catálogo diz tier e preço de lista. A sessão diz o que pode ser escrito. Sem lista, não há troca.

Decisão: não inventar modelo e não consultar a API da conta. Trade-off: se o Codex recusar `gpt-6-luna` na conta ChatGPT, o processo morre antes do hook. O Downshift não corrige o modelo pai.

**7. O spawn segue.** `rewrite_emitted` é a emissão. O spawn seguinte da mesma sessão, com `requested_model` igual ao `final_model` anterior, é o sinal de que o harness aplicou. Token e dólar só entram com `usage`.

Decisão: não tratar emissão como ACK. Trade-off: a prova completa ainda depende do harness. No Codex isso já apareceu numa sessão local. No Claude Code, a ferramenta Task não existe no plano free.

## Onde ler cada pedaço

| Doc | O que resolve |
|---|---|
| [01-entrada](01-entrada.md) | stdin, o que é tarefa, o teto de 1 MB |
| [02-classificador](02-classificador.md) | sinais, classes, Graphify |
| [03-marcha](03-marcha.md) | tier, veredito, confiança, ids do Codex |
| [04-fallbacks](04-fallbacks.md) | o que nunca bloqueia o spawn |
| [05-catalogo-e-sessao](05-catalogo-e-sessao.md) | preço de lista versus o que a conta abre |
| [06-harnesses](06-harnesses.md) | comando de cada hook |
| [07-minilm](07-minilm.md) | hash, neural, por que não é Jev |
| [08-telemetria](08-telemetria.md) | o que o JSONL pode e não pode conter |
| [09-decisoes](09-decisoes.md) | a tabela curta das escolhas |

Código: `cmd/downshift`, `internal/core`, `internal/catalog`, `internal/semantic`, `internal/telemetry`. Para ver a decisão sem harness: `downshift try "<tarefa>" codex gpt-6-luna`. A linha `Rewrite` diz se o hook trocaria o modelo.
