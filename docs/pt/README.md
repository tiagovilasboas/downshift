# Downshift

*Repositório GitHub: [`harness-downshift`](https://github.com/tiagovilasboas/downshift) · CLI `downshift` · licença Apache 2.0.*

O **Downshift** é um roteador determinístico de modelos para fluxos de trabalho com agentes de IA (hooks em harnesses de código; não é gateway HTTP tipo LiteLLM). Quando usar ou não: [WHEN-TO-USE em inglês](../when-to-use.md).

Quando você dispara um subagente em ferramentas como **Claude Code**, **Cursor**, **Codex**, **Antigravity** ou **KiroCrew**, o comportamento padrão do ambiente é fazer com que o subprocesso herde o modelo mais avançado e caro da sessão ativa (como Claude Opus ou GPT-4o/5). Na prática, gasta-se orçamento de fronteira em tarefas estritamente mecânicas — renomear variáveis, corrigir digitação, listar diretórios ou rodar comandos de terminal.

Com o Downshift, cada subagente é direcionado para a categoria de modelo ideal para a complexidade da demanda **antes da execução começar**. O desenvolvedor continua operando normalmente, sem precisar alternar modelos manualmente na interface.

O Downshift não é um chat secundário e não gera código. É um **roteador de infraestrutura**: intercepta o pedido no hook de spawn, classifica a demanda em milissegundos na CPU local, valida se a alteração é segura e, quando aplicável, reescreve o modelo do subagente de forma transparente.

---

## O Que Você Ganha

* **Eficiência Real de Tokens:** Tarefas triviais e mecânicas rodam em modelos econômicos (~75% mais baratos na tabela de preços dos provedores), reservando os modelos de fronteira para desafios arquiteturais, concorrência e alto risco.
* **Privacidade Absoluta (Zero-Leakage):** A classificação e o roteamento ocorrem 100% na máquina local. Nenhum texto de prompt, caminho de arquivo ou código sai para serviços externos ou é persistido em arquivos de log.
* **Velocidade na CPU:** Sem LLM no loop de classificação e sem chamadas de rede adicionais. A inferência híbrida (regex + centróides vetoriais) decide em menos de 2 milissegundos.
* **Segurança e Continuidade (Fail-Open):** Na dúvida ou em caso de qualquer falha técnica, o Downshift não interrompe o trabalho: o fluxo segue normalmente com o modelo original da sessão.

> **⚠️ Nota de Transparência:** `rewrite_emitted` na telemetria indica que o Downshift emitiu com sucesso a instrução de troca de modelo. Métricas em dólares reais dependem do reporte de tokens retornado pelo harness no hook posterior (`PostToolUse`). Para detalhes dos critérios de saída do beta, consulte [`docs/beta-exit.md`](../beta-exit.md).

---

## Comportamento Prático no Dia a Dia

| Tarefa do Subagente | Complexidade | Decisão do Downshift | Modelo Típico |
|---|---|---|---|
| Renomear uma variável ou corrigir typo | **TRIVIAL** | Mantém/reduz para modelo econômico | `haiku`, `luna`, `flash_lite` |
| Ajuste pontual em arquivo ou componente isolado | **SIMPLE** | Mantém no modelo econômico | `haiku`, `luna`, `flash_lite` |
| Implementar feature completa em vários arquivos | **MEDIUM** | Direciona para modelo intermediário | `sonnet`, `terra`, `flash` |
| Re-arquitetura de sistema, auth multi-tenant ou concorrência | **COMPLEX** | Direciona/mantém no modelo de fronteira | `opus`, `sol`, `pro` |
| Pedido vago ou sem sinais claros de intenção | *Ambíguo* | Não altera. Preserva o modelo original da sessão | Modelo ativo |

As faixas de capacidade permanecem equivalentes entre os ambientes suportados:
* **Codex:** `gpt-6-luna` (Small) · `gpt-5.6-terra` (Mid) · `gpt-6-sol` (Frontier)
* **Claude Code:** `claude-haiku-4-5` (Small) · `claude-sonnet-5-5` (Mid) · `claude-opus-5-5` (Frontier)
* **Antigravity:** `flash_lite` (Small) · `flash` (Mid) · `pro` (Frontier)

---

## Navegação da Documentação

A estrutura abaixo detalha cada etapa do ciclo de vida da requisição, combinando os objetivos de arquitetura de produto com os respectivos pontos de código:

| Documento | Foco | O que cobre |
|---|---|---|
| [01 · Entrada do Fluxo](01-entrada.md) | Ciclo de Vida | O que o Downshift intercepta no stdin, limites de payload e o que é ignorado |
| [02 · Classificador](02-classificador.md) | Decisão | Heurísticas de regex, matriz de sinais ponderados e o limiar de confiança |
| [03 · Gestão de Marchas](03-marcha.md) | Política | As faixas de capacidade (Small/Mid/Frontier), regras de downshift e upshift seguro |
| [04 · Fallbacks e Tolerância](04-fallbacks.md) | Resiliência | O contrato fail-open: como o sistema reage a erros sem quebrar o fluxo do dev |
| [05 · Catálogo e Sessão](05-catalogo-e-sessao.md) | Permissões | Preço de lista versus modelos realmente autorizados na sessão ativa (`session-models.json`) |
| [06 · Adapters de Harness](06-harnesses.md) | Integração | Especificidades de integração: Claude Code, Codex, Cursor, Antigravity e KiroCrew |
| [07 · Camada Semântica MiniLM](07-minilm.md) | Inteligência | Classificação local por centróides vetoriais, boost semântico e privacidade |
| [08 · Telemetria e Métricas](08-telemetria.md) | Observabilidade | Estrutura do `events.jsonl`, métricas normalizadas e auditoria sem vazamento de prompt |
| [09 · Decisões Técnicas e Trade-offs](09-decisoes.md) | Engenharia | Racional de arquitetura, compensações técnicas e próximos passos do projeto |
| [10 · Candidato em shadow](10-candidato-shadow.md) | Avaliação | Observar um candidato softmax sem trocar o roteamento de produção |

---

## Testando a Tomada de Decisão no Terminal

Você pode simular e inspecionar qualquer decisão diretamente pela linha de comando, sem precisar abrir uma sessão de agente:

```bash
# Simular uma tarefa trivial no Codex partindo de um modelo de fronteira
downshift try "renomeie a variável userId em auth.go" codex gpt-6-sol

# Simular uma tarefa complexa de arquitetura no Claude Code
downshift try "rearchitect auth module to support multi-tenant" claude-code claude-haiku-4-5
```

A saída exibe a complexidade detectada, o tier recomendado, a variação de custo estimada e a confirmação se o modelo seria reescrito pelo hook. Para o guia completo de instalação e configuração nos clientes, consulte o [README principal da raiz](../../README.md).
