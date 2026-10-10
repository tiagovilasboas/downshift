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

Se os padrões do prompt não apresentarem correspondência clara com nenhuma classe, a tarefa pode ficar com baixa confiança. Diante de incerteza, a política de roteamento opta pela segurança e **não altera** o modelo ativo.

A tabela de pesos, os pontos extras e a escolha do modelo em cada harness estão em [complexity-weights.md](../complexity-weights.md).

---

## Detalhes de Engenharia

Pontuação por regex, margem de confiança, Graphify opcional, boost MiniLM e fluxo de contribuição (`misroute:`) estão documentados em inglês para mantenedores:

**[contrib/classifier.md](../contrib/classifier.md)**

O candidato estatístico da v2 **não** substitui este caminho nos hooks. Ele só observa com `DOWNSHIFT_SHADOW_WEIGHTS`; relatório: `downshift shadow-report`. Ver [10 · Candidato em shadow](10-candidato-shadow.md).

---

## Compensações Técnicas (Trade-offs)

* **Vantagem:** Heurísticas de regex são determinísticas, auditáveis, sem GPU e rápidas na CPU.
* **Custo:** Regex captura vocabulário explícito, não intenção profunda; o boost semântico e regras direcionadas compensam casos conhecidos.
