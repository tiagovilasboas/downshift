# Downshift — Documentação Oficial em Português (PT-BR)

> Downshift por Tiago Vilas Boas  
> Repositório oficial: https://github.com/tiagovilasboas/downshift  
> Licença: Apache 2.0 · CLI: `downshift` · Go 1.27

O **Downshift** é uma ferramenta de otimização de contexto e delegação inteligente de modelos para agentes de codificação baseados em IA (como Claude Code, Codex, Cursor, OpenCode, Antigravity e KiroCrew). Ele atua diretamente na camada de infraestrutura de execução, reduzindo drasticamente o consumo desnecessário de tokens e garantindo que cada tarefa seja executada pelo modelo adequado dentro dos limites que você declarar.

Toda a lógica do Downshift roda localmente em código determinístico Go de altíssimo desempenho: **o código do Downshift nunca faz chamadas a LLMs, não grava prompts ou códigos em disco para telemetria, não envia dados para a rede e opera em modo *fail-open* (em caso de dúvida ou erro, a execução original é sempre preservada sem bloqueios).**

---

## Sumário

- [Visão Geral e Propósito](#visão-geral-e-propósito)
- [Instalação e Configuração Rápida](#instalação-e-configuração-rápida)
- [Status de Suporte por Harness](#status-de-suporte-por-harness)
- [Resumo dos Benchmarks e Economia de Tokens](#resumo-dos-benchmarks-e-economia-de-tokens)
- [Compactor e CCR (Command Context Recovery)](#compactor-e-ccr-command-context-recovery)
- [Roteamento e Delegação de Modelos por Subagente](#roteamento-e-delegação-de-modelos-por-subagente)
- [Orquestração de Agentes e Gatilhos de Contexto](#orquestração-de-agentes-e-gatilhos-de-contexto)
- [Comandos Essenciais do CLI](#comandos-essenciais-do-cli)
- [Navegação da Documentação PT-BR](#navegação-da-documentação-pt-br)
- [Segurança, Privacidade e Governança](#segurança-privacidade-e-governança)

---

## Visão Geral e Propósito

Agentes de codificação autônomos desperdiçam grande parte de suas janelas de contexto consumindo saídas gigantescas de comandos de terminal (como suítes de testes que passam com sucesso ou logs repetitivos de build) e delegando subtarefas mecânicas e simples aos modelos mais caros e lentos da categoria *frontier*.

O Downshift resolve esses dois desafios por meio de dois pilares complementares:

1. **Compactor com CCR sem perda de informação:**  
   Intercepta comandos permitidos (como `go test`, `vitest`, `pytest`, `cargo test`, `git status`, `npm install`, etc.) e resume execuções bem-sucedidas a sínteses concisas de `PASS/FAIL`. Com o sistema de CCR (*Command Context Recovery*) ativado por padrão, a saída bruta integral é gravada em um buffer seguro local (`~/.downshift/ccr/<id>.txt`), permitindo que o agente recupere qualquer trecho ou stack trace instantaneamente com `downshift retrieve <id>`. Em execuções com falha, 100% dos erros, asserções e stack traces do código da aplicação são preservados na íntegra.

2. **Roteamento e Delegação de Modelos por Subagente:**  
   O orquestrador líder da sessão decide o modelo e nível de esforço adequados para cada subagente delegado. O Downshift intercepta o evento de disparo via hook nativo (`PreToolUse`), valida a decisão contra as políticas declaradas em `policy.json` (pisos de modelo, travas de cota e regras por tipo de tarefa) e reescreve a chamada em tempo de execução. Trabalho mecânico ou simples é executado em modelos menores (*small*), economizando tokens e preservando o cache de prompt da sessão principal. Na dúvida, o modelo original solicitado pelo desenvolvedor é mantido inalterado.

---

## Instalação e Configuração Rápida

A instalação é feita em três comandos simples:

```bash
# 1. Baixar e instalar o binário oficial
curl -fsSL https://raw.githubusercontent.com/tiagovilasboas/downshift/main/install.sh | sh

# 2. Configurar os harnesses instalados na máquina (assistente interativo seguro)
downshift setup

# 3. Validar a integridade da instalação, hooks e permissões
downshift doctor
```

- O assistente `downshift setup` detecta automaticamente os agentes e harnesses suportados presentes no sistema, exibe um diff claro de qualquer alteração de configuração e pede confirmação antes de gravar.
- O utilitário `downshift doctor` executa um checklist rigoroso do binário, hooks ativos, diretórios de estado e conectividade de MCP, oferecendo comandos de autocorreção caso encontre inconsistências.

Para o passo a passo completo, consulte o [Guia de Início Rápido](guias/inicio-rapido.md).

---

## Status de Suporte por Harness

O Downshift foi projetado segundo a regra de camadas arquiteturais: a inteligência vive nas camadas compartilhadas e os adapters traduzem os eventos nativos de cada harness.

| Harness | Compactação | Roteamento de Modelos | Status Oficial | Modo de Configuração |
|---|---|---|---|---|
| **Claude Code** | Reescrita automática de comandos | Reescrita de modelo observada e honrada | **Completed** | `downshift setup claude-code` |
| **Codex** | Execução direta via `downshift compact` | Reescrita emitida e honrada inferida | **Functional** | `downshift setup codex` |
| **Cursor** | Execução direta via `downshift compact` | Reescrita emitida e honrada não confirmada | **Functional** | `downshift setup cursor` |
| **OpenCode** | Reescrita automática via bridge plugin | Reescrita observada e honrada | **Beta** | `downshift setup opencode` |
| **Antigravity** | Execução direta via `downshift compact` | Suporte a rotas e subagentes locais | **Functional** | `downshift setup antigravity` |
| **KiroCrew** | Execução direta via `downshift compact` | Modo de política (allow/block, sem reescrita in-place) | **Experimental** | `downshift setup kirocrew` |

> **Nota:** A compactação direta (`downshift compact -- <comando>`) funciona de maneira universal em **qualquer terminal e harness** (suportado ou não), pois opera como um comando shell padrão independente de hooks.

---

## Resumo dos Benchmarks e Economia de Tokens

Os ganhos do Downshift são medidos em bytes e contagens de tokens em um corpus de teste com **107 fixtures sintéticas** representativas do ciclo real de desenvolvimento:

| Métrica Avaliada | Resultado Obtido | Significado Técnico |
|---|---|---|
| **Corpus Geral (CCR desligado)** | **51.1%** de economia em bytes | Redução sem compactação agressiva de repetições |
| **Corpus Geral (CCR ativado)** | **72.0%** de economia em bytes | **Padrão atual:** máxima redução com recuperação integral |
| **Suítes Lossless (Testes)** | **65.2%** de economia em bytes | Preserva 100% de erros e stack traces da aplicação |
| **Picos de Redução** | **80% a 95%** de economia | *Progress* (94.5%), `cargo test` (93.0%), `flutter test` (92.3%), *lockfile diff* (91.2%), `npm install` (89.1%), `pytest` (86.7%), `playwright` (81.6%), `turbo` (79.6%), `tsc` (78.4%), `vitest` (74.3%) |

Em test runners *lossless*, o compilador e o framework de testes continuam gerando suas mensagens de falha completas: o Downshift corta apenas as linhas repetidas de testes que passaram com sucesso (`PASS`, `ok`), evidenciando imediatamente as falhas.

Consulte a análise técnica aprofundada na [Referência de Benchmarks](referencia/benchmarks.md).

---

## Compactor e CCR (Command Context Recovery)

O compactor envolve comandos verborrágicos mantendo integralmente o código de saída (*exit code*):

```bash
downshift compact -- go test ./...
downshift compact -- git status
```

### Como Funciona o CCR

A partir de 6 de novembro de 2026, o CCR vem **ativado por padrão**:
1. Toda saída bruta gerada é armazenada de forma segura em `~/.downshift/ccr/<id>.txt`.
2. O compactor insere um marcador transparente ao final da saída compactada:
   `[downshift compact: compacted 3956 to 998 bytes; full output: downshift retrieve 3f9a1c2b7d40]`
3. Sequências de 8 ou mais linhas repetitivas ou idênticas são dobradas (exibindo as 3 primeiras e as 2 últimas, com `[... N similar lines folded]`).
4. Se o agente ou o desenvolvedor precisar inspecionar a saída bruta completa ou linhas específicas, basta executar:

```bash
# Recuperar saída completa
downshift retrieve 3f9a1c2b7d40

# Filtrar por expressão regular ou intervalo de linhas
downshift retrieve 3f9a1c2b7d40 --grep "FAIL|panic"
downshift retrieve 3f9a1c2b7d40 --lines 50:120
```

Para detalhes de sanitização de credenciais e retenção de arquivos, veja o [Guia de Compactação e CCR](guias/compactacao-e-ccr.md).

---

## Roteamento e Delegação de Modelos por Subagente

Quando a sessão orquestradora delega trabalho a um subagente, o Downshift avalia a solicitação em milissegundos seguindo uma ordem estrita de precedência (a primeira regra que der *match* vence):

```mermaid
flowchart TD
    Req["Spawn de Subagente Solicitado"] --> Pinned{"Modelo fixado (pinned)?"}
    Pinned -- Sim --> Keep["Mantém Modelo Solicitado"]
    Pinned -- Não --> Profile{"Perfil declarado em policy.json?"}
    Profile -- Sim --> ApplyProfile["Aplica Perfil"]
    Profile -- Não --> Rules{"Regra declarada (kind/effort/quota)?"}
    Rules -- Sim --> ApplyRule["Aplica Regra Declarada"]
    Rules -- Não --> Hint{"Orquestrador sugeriu tier menor?"}
    Hint -- Sim --> ApplyHint["Desce para o Tier Indicado"]
    Hint -- Não --> TaskClass{"Classe da Tarefa é TRIVIAL/SIMPLE sem risco?"}
    TaskClass -- Sim --> ApplySmall["Desce para Small"]
    TaskClass -- Não --> Keep
```

- **Dúvida mantém o modelo original:** Se a solicitação não puder ser classificada com alta confiança, o Downshift não altera o modelo solicitado (princípio *fail-open*).
- **Sem elevação não autorizada:** O Downshift nunca eleva o modelo além do solicitado pelo orquestrador, salvo se `routing.raise_complex: true` for explicitamente configurado no `policy.json`.
- **Integridade do cache de prompt:** O modelo da sessão principal do usuário nunca é alterado no meio do turno, garantindo 100% de reaproveitamento do *prompt cache*.

Veja todos os detalhes no [Guia de Roteamento de Modelos](guias/roteamento-de-modelos.md).

---

## Orquestração de Agentes e Gatilhos de Contexto

O Downshift acompanha um squad com 14 papéis de agentes especializados em 6 domínios de engenharia:
- **Core:** `coder`, `coder-back`, `coder-front`, `refactor`, `researcher`
- **Quality:** `tester`, `debugger`, `reviewer`
- **Security:** `security`
- **Performance:** `performance`
- **Support:** `documenter`
- **Leadership:** `architect`, `platform`, `ai`

### Gatilhos de Contexto (Context Triggers)

Você pode invocar o esquadrão no chat do seu agente usando gatilhos em linguagem natural:

```
Usuário: Preciso refatorar o pool de conexões do banco de dados e cobrir com testes
Usuário: downshift full-power
```

- `downshift full-power` (ou seus aliases em português `potência total` / `potencia-total`): Ativa os especialistas necessários para a tarefa atual (respeitando o limite de até 5 agentes simultâneos e checando as cotas de consumo antes de despachar).
- `downshift shift <squad>`: Ativa um domínio específico de engenharia (`core`, `quality`, `security`, `leadership`). Exemplos:
  - `downshift shift quality`
  - `downshift shift security`
  - `downshift shift coder "corrigir race condition no worker pool"`

---

## Comandos Essenciais do CLI

| Comando | Descrição e Finalidade |
|---|---|
| `downshift setup [--dry-run]` | Configura hooks nos harnesses detectados com diff prévio e confirmação |
| `downshift doctor` | Diagnóstico de integridade do ambiente com comandos de correção automática |
| `downshift compact -- <cmd>` | Executa um comando permitido e compacta a saída mantendo status e saída de erro |
| `downshift retrieve <id>` | Recupera a saída bruta mantida no buffer seguro do CCR |
| `downshift session [--why]` | Exibe a árvore de subagentes da sessão e a justificativa clara de cada decisão |
| `downshift stats` | Apresenta resumo de decisões tomadas e estimativa de tokens economizados |
| `downshift report` | Relatório consolidado de tokens direcionados por tarefa concluída |
| `downshift policy check` | Valida a sintaxe e a semântica do arquivo de política `policy.json` |
| `downshift quota` | Diagnóstico das cotas e janelas de consumo dos provedores configurados |
| `downshift mcp serve` | Inicia o servidor MCP local (expondo 13 ferramentas nativas para agentes) |

---

## Navegação da Documentação PT-BR

Para aprofundar seu conhecimento em cada área do Downshift, explore os guias e referências dedicados:

- 🚀 [Guia de Início Rápido](guias/inicio-rapido.md) — Instalação passo a passo, primeiro comando compactado e testes.
- 📦 [Guia de Compactação e CCR](guias/compactacao-e-ccr.md) — Funcionamento interno do compactor, ferramentas suportadas e recuperação de logs via CCR.
- 🔀 [Guia de Roteamento de Modelos](guias/roteamento-de-modelos.md) — Estrutura de regras BDI, guardrails, `policy.json` e otimização por subagente.
- 📊 [Referência de Benchmarks](referencia/benchmarks.md) — Metodologia de testes, tabela oficial detalhada de 107 fixtures e escada de evidências.

---

## Segurança, Privacidade e Governança

- **Fail-Open Garantido:** Qualquer erro inesperado, payload corrompido, timeout ou tamanho superior a 1 MiB faz o hook liberar a execução original imediatamente sem alteração.
- **Privacidade de Prompts e Código:** O Downshift nunca salva textos de prompts, arquivos do repositório ou parâmetros confidenciais em sua telemetria. O arquivo `events.jsonl` armazena apenas contagens, hashes e tempos.
- **Sanitização Heurística no CCR:** O buffer de CCR filtra ativamente linhas com padrões de credenciais, chaves de API, tokens JWT e blocos PEM. O armazenamento é local, com permissões restritas (0600/0700) e expiração automática em 24 horas.
- **Sem Portas dos Fundos ou LLMs Ocultos:** Toda decisão é tomada por código Go estático e determinístico inspecionável em código aberto.
