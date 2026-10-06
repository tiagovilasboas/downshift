# 02 · Classificador

O classificador resolve a questão central da delegação: **esta tarefa é um trabalho mecânico trivial, uma alteração isolada de código, uma funcionalidade completa ou o desenho de um sistema de alta complexidade?**

---

## Perspectiva de Produto

A taxonomia de complexidade do Downshift é dividida em quatro classes bem delimitadas:

| Classe | Natureza da Tarefa | Exemplos Práticos |
|---|---|---|
| **TRIVIAL** | Trabalho mecânico de baixo risco | Renomear variável, formatar arquivo, ajustar `git status`, corrigir typo no README |
| **SIMPLE** | Alteração pontual e isolada | Adicionar campo em struct, criar cena básica (ex: cubo Three.js), editar um arquivo |
| **MEDIUM** | Implementação de funcionalidade | Criação de export CSV, fluxo de validação tocando múltiplos arquivos |
| **COMPLEX** | Arquitetura, concorrência e infraestrutura | Re-arquitetura multi-tenant, service mesh, eleição de líder (Raft/etcd), migração de banco |

Se os padrões do prompt não apresentarem correspondência clara com nenhuma classe, a tarefa é classificada preventivamente como **`MEDIUM`** com indicador de baixa confiança. Diante de incerteza, a política de roteamento opta pela segurança e **não altera** o modelo ativo.

---

## Detalhes de Engenharia

A primeira linha de classificação opera em `internal/core/signals.go`, através de um sistema de pontuação ponderada por expressões regulares:

1. **Votação Ponderada:** Cada regex possui um peso associado. A classe com o maior somatório de pontos vence a disputa.
2. **Desempate Seguro:** Em caso de empate entre classes vizinhas, a regra do sistema sempre prioriza o tier de maior capacidade (nunca subdimensiona).
3. **Margem de Confiança:** Para uma classificação ser considerada "confiante", a classe vencedora precisa abrir pelo menos **2 pontos de vantagem** sobre a segunda colocada.
   * Exemplo: Um termo genérico como `implement` soma 2 pontos em `MEDIUM`. No entanto, `three.js scene` soma 3 pontos em `SIMPLE`, superando o verbo genérico. Da mesma forma, termos como `service mesh` ou `raft` somam 3 pontos em `COMPLEX`, vencendo a disputa com folga.
4. **Graphify (Opcional):** Sem `DOWNSHIFT_GRAPHIFY_CMD`, o fetcher é nulo. Com o comando, o label vai no stdin e o JSON do nó volta no stdout; falha, timeout ou JSON inválido não sobem a classe. `DOWNSHIFT_GRAPHIFY=0` desliga o fetcher.
5. **Boost Semântico (MiniLM):** Localizado em `internal/semantic`, este módulo calcula distâncias de centróides em espaço vetorial. Ele é estritamente monotônico: pode confirmar ou elevar a classe quando o regex está hesitante, mas **nunca rebaixa**. Pode ser desativado via `DOWNSHIFT_MINILM=0`.

---

## Compensações Técnicas (Trade-offs)

* **Vantagem:** Heurísticas de regex são determinísticas, 100% auditáveis, livres de GPU e executam em frações de milissegundo.
* **Custo:** Regex avalia vocabulário e padrões explícitos, não intenção latente profunda. Por essa razão, termos como cubos interativos ou protocolos distribuídos contam com regras semânticas direcionadas para garantir que a categoria correta seja atribuída.

O candidato estatístico da v2 fica fora deste caminho. Ele só observa, com `DOWNSHIFT_SHADOW_WEIGHTS`, e o relatório é `downshift shadow-report`. Ver [10 · Candidato em shadow](10-candidato-shadow.md).
