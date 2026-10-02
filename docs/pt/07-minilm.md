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
   * Caso o desenvolvedor aponte a variável `DOWNSHIFT_MINILM_EMBED` para um script (ex: `python3 tools/minilm/embed_stdin.py`), o Downshift utiliza embeddings gerados pelo modelo `sentence-transformers/all-MiniLM-L6-v2`.
   * Centróides neurais em `internal/semantic/data/minilm.json`.
   * **Resultados no Benchmark Holdout (300 tarefas inéditas):** O classificador neural atingiu **98.0% de acurácia de tier**, com **0% de erros críticos** do tipo `FRONTIER → MID` (ou seja, tarefas de alta complexidade nunca foram rebaixadas indevidamente).
   * Caso o comando externo falhe ou demore, o sistema faz fallback imediato para os centróides de hash locais sem travar a thread.

---

## Compensações Técnicas (Trade-offs)

* **Vantagem:** Evita custos por token para classificar tarefas, elimina dependência de conexão de rede externa e mantém latência mediana na faixa de ~1.8 ms em CPU simples.
* **Custo:** Centróides locais por similaridade operam melhor em vocabulário técnico consistente (inglês/código). Tarefas em outros idiomas ou fora do domínio de desenvolvimento de software dependem primariamente das heurísticas regex do sistema.
