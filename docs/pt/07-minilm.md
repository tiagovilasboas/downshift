# MiniLM e a decisão local

O Downshift não usa o Jev no caminho crítico. O Jev é um modelo de decisão remoto: manda o texto para uma API. O contrato daqui é o contrário: sem rede, sem chave, sem texto saindo da máquina.

O que roda no binário:

- Regex, sempre.
- Hash embutido (`internal/semantic/data/prototypes.json`), ligado por padrão. Compara o vetor do prompt com centroides das quatro classes e só sobe a classe.
- MiniLM neural (`all-MiniLM-L6-v2`) só se `DOWNSHIFT_MINILM_EMBED` apontar para um comando. Os centroides estão em `internal/semantic/data/minilm.json`. Se o comando falha, o hash assume.

Medição no holdout de 300 tarefas, centroides treinados só em `benchmark/tasks.json` (`benchmark/minilm-holdout.json`): o centroide neural fez 96,3% de acerto de tier e 0% de FRONTIER→MID. O regex, na mesma época, estava na casa dos 98% com alguns FRONTIER→MID. O neural não virou o padrão porque não ganhou acerto de tier e exige Python. O padrão continua o hash, dentro do processo.

`downshift try` é o jeito de ver a decisão sem harness. A linha `Rewrite: yes/no` diz se o hook trocaria o modelo, não só qual foi o veredito.
