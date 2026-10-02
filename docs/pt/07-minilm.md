# MiniLM e a decisão local

A triagem não sai da máquina. Não é um SLM escrevendo texto e não é o Jev numa API.

**Produto.** O Downshift decide com o que já está no binário. O texto da tarefa não vai para um classificador pago. Se a camada extra falhar, o regex permanece. Você desliga tudo com `DOWNSHIFT_MINILM=0`.

**Técnico.** Regex sempre. Hash embutido (`internal/semantic/data/prototypes.json`) ligado por padrão: vetor do prompt contra centroides das quatro classes, só sobe. MiniLM neural (`all-MiniLM-L6-v2`) só com `DOWNSHIFT_MINILM_EMBED` apontando para um comando; centroides em `internal/semantic/data/minilm.json`. Comando falhou, hash assume. Holdout de 300 tarefas, treino só em `tasks.json`: neural 96,3% de acerto de tier e 0% FRONTIER→MID. Regex, na mesma época, na casa dos 98% com alguns FRONTIER→MID. Por isso o neural não é o padrão.

`downshift try` mostra a decisão. `Rewrite: yes/no` é o que o hook faria, não só o veredito.

**Trade-off.** Jev acertaria classe com um serviço remoto e um protocolo de decisão. Quebraria o “nada sai da máquina” e o custo zero da triagem. Hash cabe no binário e erra diferente do regex: às vezes sobe um cubo para `MEDIUM` sem confiança. O freio da marcha segura o modelo barato.
