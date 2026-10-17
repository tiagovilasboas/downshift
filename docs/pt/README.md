# Downshift

*Repositório GitHub: [`downshift`](https://github.com/tiagovilasboas/downshift) · CLI `downshift` · licença Apache 2.0.*

O **Downshift** é um **otimizador de custo e eficiência de execução para agentes de IA**. Ele se instala nos hooks do seu harness de código e atua onde o custo é decidido: quando o agente principal delega. Por padrão, um subagente herda o modelo mais caro da sessão, então renomear uma variável, rodar um teste ou listar um diretório sai a preço de fronteira, e uma sessão que delega muito multiplica isso.

O roteamento de modelos é a alavanca principal hoje, não o produto inteiro. Em volta dele: escolha conservadora na dúvida (um palpite nunca gasta dinheiro), guardrails para tarefas de risco e piso de política por projeto, visibilidade de cada decisão, telemetria do que cada filho custou e como terminou, grupo de controle e `downshift report` para medir custo por tarefa concluída, skills de diagnóstico e companions de contexto. Tudo determinístico (nenhum LLM no loop), sem guardar prompt e fail-open. A métrica que importa é **custo por tarefa concluída com qualidade aceitável**; token é proxy. Tabela completa com status e evidência por capacidade: [capability matrix (EN)](../reference/capability-matrix.md).

Quando você dispara um subagente em ferramentas como **Claude Code**, **Codex** ou **Cursor** (os harnesses Completed e Functional, abaixo), cada um é direcionado para a categoria de modelo adequada à complexidade da demanda **antes da execução começar**, e só quando o classificador está confiante e nenhum guardrail segura. O desenvolvedor continua operando normalmente.

Toda funcionalidade é construída nas camadas compartilhadas e calculada para todos os harnesses. **O Claude Code é o harness principal**: é onde o Downshift está mais completo (modelo da sessão inferido, hooks de uso e de fim do filho, comando `/downshift`, statusline, política de sessão com `opusplan`) e mais verificado. Os demais rodam o mesmo núcleo; a [matriz de harnesses](../reference/harness-matrix.md) diz o que cada um honra hoje, e o [guia do Claude Code (EN)](../guides/claude-code.md) explica a relação com o `opusplan`.

**Status por harness.** Cada harness tem exatamente uma de três palavras de status, rastreável a um arquivo de evidência ou a um teste (a suíte `cmd/downshift/conformance_test.go` roda os mesmos 8 cenários de roteamento contra todo hook registrado). Detalhes por harness: [matriz de harnesses (EN)](../reference/harness-matrix.md).

| Harness | Status | O que isso significa | Evidência |
|---|---|---|---|
| **Claude Code** | Completed | Reescrita observada como honrada em um filho real, uso e fim do filho registrados, plugin e política de sessão | [reescrita honrada](../evidence/claude-code-rewrite-honored-2026-10-06.md) · [auditoria de cobrança](../evidence/billing-gap-audit.md) · [A/B de roteamento](../evidence/subagent-routing-ab-2026-10-15.md) |
| **Codex** | Functional | O hook emite a reescrita em um spawn real; que o harness a aplicou é inferido, não observado; sem uso do filho | [Codex](../evidence/codex-rewrite-honored-2026-10-02.md) |
| **Cursor** | Functional | O hook emite a reescrita em um spawn real; que o harness a aplicou não foi confirmado; sem uso do filho | [Cursor](../evidence/cursor-rewrite-honored-2026-10-10.md) · [diagnóstico](../evidence/cursor-gap-diagnostic.md) |

**Experimental (prova de conceito).** Antigravity e KiroCrew têm um filho observado em um build cada (KiroCrew em modo política: bloqueia e pede novo spawn, sem reescrita no lugar) e nenhum fim de filho registrado. Os adapters, testes e hooks continuam no repositório e calculam todas as funcionalidades; a instalação está no [guia de instalação (EN)](../guides/install.md#experimental-harnesses). Evidência: [Antigravity](../evidence/antigravity-executor-ack.md) · [KiroCrew](../evidence/kirocrew-native-hook-2026-10-10.md). O Grok CLI continua apenas por configuração, sem reescrita por hook.

O Downshift não é um chat secundário, não gera código, não é proxy HTTP e não altera a saída de ferramentas em voo. É uma **camada de otimização de infraestrutura**: intercepta o pedido no hook de spawn, classifica a demanda em milissegundos na CPU local, valida se a mudança é segura e, quando aplicável, reescreve o modelo do subagente de forma transparente.

**O que está provado e o que não está.** Um A/B controlado em 2026-10-15, um operador, Claude Code, mesmas tarefas em braço roteado e braço de controle: em peças mecânicas entregues uma por subagente (o teto) o custo caiu cerca de 12% com os mesmos 12 de 12 aceitos; em tarefas naturais a maioria dos spawns foi segurada e a diferença de custo não é atribuível ao roteamento. A [escada de evidência (EN)](../guides/evidence.md) separa teoria de tabela, estimativa de telemetria, benchmark controlado e resultado em tarefa real; custo por tarefa concluída foi medido uma vez, não ainda em carga natural.

---

## O Que Você Ganha

* **Eficiência Real de Tokens:** Tarefas triviais e mecânicas rodam em modelos econômicos (~75% mais baratos que o Opus na tabela de preços dos provedores), reservando os modelos de fronteira para desafios arquiteturais, concorrência e alto risco.
* **Privacidade Absoluta (Zero-Leakage):** A classificação e o roteamento ocorrem 100% na máquina local. Nenhum texto de prompt, caminho de arquivo ou código sai para serviços externos ou é persistido em arquivos de log.
* **Velocidade na CPU:** Sem LLM no loop de classificação e sem chamadas de rede adicionais. A inferência (regex mais um score semântico local por hash de palavras; MiniLM é opcional) decide em menos de 1 ms em prompts curtos e em cerca de 0,2 s num prompt de 1 MiB.
* **Segurança e Continuidade (Fail-Open):** Na dúvida ou em caso de qualquer falha técnica, o Downshift não interrompe o trabalho: o fluxo segue normalmente com o modelo original da sessão.

> **⚠️ Nota de Transparência:** `rewrite_emitted` na telemetria indica que o Downshift emitiu com sucesso a instrução de troca de modelo. Métricas em dólares reais dependem dos tokens do filho, que no Claude Code vêm do transcript lido no `SubagentStop` (o `PostToolUse` traz o modelo resolvido, não tokens) e são precificados pela tabela do catálogo, não pela fatura. Para detalhes dos critérios de saída do beta, consulte [`docs/reference/beta-exit.md`](../reference/beta-exit.md).

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
* **Antigravity (Experimental):** `flash_lite` (Small) · `flash` (Mid) · `pro` (Frontier)

---

## Navegação da Documentação

A estrutura abaixo detalha cada etapa do ciclo de vida da requisição, combinando os objetivos de arquitetura de produto com os respectivos pontos de código:

| Documento | Foco | O que cobre |
|---|---|---|
| [01 · Entrada do Fluxo](01-entrada.md) | Ciclo de Vida | O que o Downshift intercepta no stdin, limites de payload e o que é ignorado |
| [02 · Classificador](02-classificador.md) | Decisão | Heurísticas de regex, matriz de sinais ponderados e o limiar de confiança |
| [Visão BDI agentic (EN)](../README.md#bdi-agentic-view-beliefs--intentions) | Arquitetura | Diagrama completo: percepção → crenças → desejos → deliberação → intenções → ação (todos os harnesses; sem LLM no loop) |
| [03 · Gestão de Marchas](03-marcha.md) | Política | As faixas de capacidade (Small/Mid/Frontier), regras de downshift e upshift seguro |
| [04 · Fallbacks e Tolerância](04-fallbacks.md) | Resiliência | O contrato fail-open: como o sistema reage a erros sem quebrar o fluxo do dev |
| [05 · Catálogo e Sessão](05-catalogo-e-sessao.md) | Permissões | Preço de lista versus modelos realmente autorizados na sessão ativa (`session-models.json`) e, desde `downshift models discover`, os modelos que o próprio harness informa ([session-discovery](../reference/session-discovery.md), em inglês) |
| [06 · Adapters de Harness](06-harnesses.md) | Integração | Especificidades de integração: Claude Code, Codex e Cursor; Antigravity e KiroCrew (Experimental) |
| [07 · Camada Semântica MiniLM](07-minilm.md) | Inteligência | Classificação local por centróides vetoriais, boost semântico e privacidade |
| [08 · Telemetria e Métricas](08-telemetria.md) | Observabilidade | Estrutura do `events.jsonl`, métricas normalizadas e auditoria sem vazamento de prompt |
| [09 · Decisões Técnicas e Trade-offs](09-decisoes.md) | Engenharia | Racional de arquitetura, compensações técnicas e próximos passos do projeto |
| [10 · Candidato em shadow](10-candidato-shadow.md) | Avaliação | Observar um candidato softmax sem trocar o roteamento de produção |
| [11 · Compressor Nativo de Contexto](../design/context-optimization.md) | Otimização | Compressor determinístico em Go e sensor de telemetria passiva |
| [12 · Plugin, skills e agente (EN)](../design/architecture.md#packaging-layers-what-an-agent-harness-loads) | Empacotamento | As cinco camadas que um harness carrega: memória (`AGENTS.md`), skills (`skills/`), hooks (binário; `hooks/hooks.json` no plugin), agente de exemplo (`agents/`) e o plugin do Claude Code (`/plugin install downshift --marketplace tiagovilasboas/downshift`) |
| [13 · Companions (EN)](../guides/companions.md) | Ecossistema | Headroom como companion padrão (detectado e registrado, nunca instalado pelo Downshift), OmniRoute como gateway opcional, mods do Claude Code recomendados (You should know, Token Weather, Replay Theater, reflect-mod), padrões que o Downshift segue mas não empacota |

---

## Relatórios e Auditoria de Economia

Para auditar o histórico de rotas, ver a estimativa de custos e acompanhar as métricas locais de tokens:

```bash
# Relatório consolidado dos últimos 30 dias
downshift stats

# Filtrar janela temporal (ex: 7 dias)
downshift stats --days=7

# Resumo em JSON higienizado para auditorias (sem texto de prompt nem caminhos)
downshift stats --export

# Iniciar dashboard web local
downshift serve
```

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
