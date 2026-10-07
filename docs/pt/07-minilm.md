# 07 · MiniLM e a Camada Semântica Local

A triagem de complexidade no Downshift é executada inteiramente dentro da máquina local. Não utilizamos modelos generativos intermediários caros, chamadas de API externas ou serviços remotos de classificação (como Jev ou Laya via nuvem).

---

## Perspectiva de Produto

* **Custo Zero e Autonomia:** Toda a inteligência necessária para avaliar o prompt vem pré-instalada no binário do Downshift. Nenhuma palavra do seu código ou da sua instrução sai para servidores de terceiros.
* **Resiliência:** Se a camada semântica vetorial encontrar qualquer anomalia de cálculo, o sistema recorre instantaneamente às regras determinísticas de regex. A camada pode ser desativada a qualquer momento definindo `DOWNSHIFT_MINILM=0`.

---

## Detalhes de Engenharia

O módulo semântico opera sob um pipeline híbrido e escalonado em `internal/semantic`:

1. **Centróides por Hash (Padrão Embarcado):**
   * Centróides pré-computados ficam embutidos no binário (`internal/semantic/data/prototypes.json`).
   * O texto do prompt é transformado em vetor de características determinísticas e comparado contra os quatro centróides de classe (`TRIVIAL`, `SIMPLE`, `MEDIUM`, `COMPLEX`).
   * Operação **estritamente monotônica**: a similaridade vetorial só é utilizada para **elevar** a classe quando o classificador de regex está indeciso; ela **nunca** rebaixa uma classificação.
2. **Modo Neural Opcional (MiniLM Real):**
   * Por padrão (`DOWNSHIFT_MINILM_EMBED` vazia ou igual a `hash`), o Downshift usa apenas o embedding por hash embutido no binário, sem Python.
   * Para ativar o modo neural, aponte `DOWNSHIFT_MINILM_EMBED` para um comando seu que gere embeddings com o modelo `sentence-transformers/all-MiniLM-L6-v2` (ex: `python3 /caminho/para/embed.py`). Este repositório não traz esse comando: os scripts de treino e avaliação ficam no repositório privado `downshift-labs`.
   * Contrato do comando: a linha é dividida por espaços e executada sem shell (sem pipes, aspas ou expansão de variáveis); o prompt chega pelo stdin; o stdout deve ser um array JSON de floats com 384 dimensões, a mesma dimensão de `minilm.json` (vetores de outro tamanho não geram boost). O stderr é ignorado.
   * Centróides neurais em `internal/semantic/data/minilm.json`.
   * **Resultados no holdout (`benchmark/minilm-holdout.json`, 300 tarefas):** o modelo `sentence-transformers/all-MiniLM-L6-v2`, treinado apenas em `benchmark/tasks.json`, atingiu **96.3% de acurácia de tier** (`tier_accuracy` 0.963333) e **0% de `FRONTIER → MID`**. A linha neural não supera a rede de regressão por regex nesse mesmo arquivo. A acurácia de 100% da regex foi medida depois que os sinais foram ajustados contra o holdout. Esse conjunto está queimado: é rede de regressão, não prova de generalização.
   * Caso o comando falhe, passe de 1 s, não devolva um array JSON válido ou devolva um vetor vazio, o sistema volta automaticamente para o embedding por hash, sem travar o hook.

---

## Compensações Técnicas (Trade-offs)

* **Vantagem:** Evita custos por token para classificar tarefas, elimina dependência de conexão de rede externa e mantém latência mediana na faixa de ~1.8 ms em CPU simples.
* **Custo:** Centróides locais por similaridade operam melhor em vocabulário técnico consistente (inglês/código). Tarefas em outros idiomas ou fora do domínio de desenvolvimento de software dependem primariamente das heurísticas regex do sistema.

Este boost entra no `core.Route`. O candidato softmax da v2 é outra coisa: só observa, e está em [10 · Candidato em shadow](10-candidato-shadow.md).
