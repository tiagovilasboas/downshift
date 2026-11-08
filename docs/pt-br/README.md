<div align="center">
<picture>
  <source media="(prefers-color-scheme: dark)" srcset="../brand/downshift-mark-darkbg.svg">
  <img src="../brand/downshift-mark-lightbg.svg" alt="Símbolo do Downshift" width="90">
</picture>

# Downshift

### Menos ruído nos comandos. Roteamento de subagentes com sua política.

Downshift é uma CLI open source para agentes de programação com IA. Ajuda a reduzir o consumo de tokens de LLM ao compactar a saída de comandos e aplica roteamento de modelos por subagente dentro da sua política. Um único binário Go.

Funciona com harnesses de programação, incluindo Claude Code e Codex CLI; as capacidades de integração e os modelos disponíveis variam por harness e sessão.

[Comece aqui](#comece-aqui) · [Documentação](../guides/install.md) · [Suporte por harness](../reference/harness-matrix.md) · [Evidências](#o-que-as-evidências-mostram) · [Como contribuir](../../CONTRIBUTING.md)

[English](../../README.md) · [Português (Brasil)](README.md)

**Open source · Apache 2.0 · Maturidade do produto: beta**
</div>

![Agentes do Downshift trabalhando juntos sobre um fundo escuro](../brand/readme-media/hero.png)

## Comece aqui

Instale um binário de release compatível, escolha seu harness, revise a configuração proposta e verifique a instalação. Para o roteamento integrado, o harness compatível precisa estar instalado; o compactor também pode ser executado diretamente como CLI independente. O assistente interativo pode configurar hooks, MCP, skills e política de sessão conforme suas escolhas. Ele mostra as alterações propostas e pede confirmação antes de gravar.

Os pacotes de release atendem macOS e Linux em arm64 e amd64, e Windows em amd64. A disponibilidade dos arquivos varia conforme o release; confira a página de releases para sua plataforma e os arquivos de verificação. O instalador shell do guia atende macOS e Linux.

[Releases](https://github.com/tiagovilasboas/downshift-ai/releases) · [Instalação](../guides/install.md) · [Início rápido](../guides/quickstart.md) · [Configuração](../guides/config.md) · [Desfazer configuração](../reference/cli.md#downshift-setup) · [Desinstalar](../reference/cli.md#downshift-uninstall)

```bash
# macOS & Linux (arm64 / amd64)
curl -fsSL https://raw.githubusercontent.com/tiagovilasboas/downshift-ai/main/install.sh | bash

# No seu repositório:
downshift setup
downshift doctor
```

[![O assistente interativo detecta harnesses, permite escolher componentes e revisar alterações, grava após a confirmação e depois o usuário verifica a configuração](../brand/readme-media/setup-flow.svg)](../brand/readme-media/setup-flow.png)

## Otimização de tokens para agentes de programação

Para desenvolvedores que utilizam agentes de código com IA, suítes longas de testes e saídas verbosas de build não apenas gastam tokens — elas diluem a janela de contexto. À medida que o histórico de terminal se acumula, a capacidade do modelo de reter instruções anteriores, invariantes de arquitetura e regras de negócio se degrada. O Downshift compacta a saída de comandos suportados antes de ela chegar ao agente, preservando o foco da memória de trabalho, códigos de saída e detalhes de falhas intactos. Saídas menores protegem a capacidade do contexto e reduzem o consumo de tokens a cada turno.

O Downshift também aplica escolhas de modelo e nível de esforço de raciocínio ao trabalho delegado, dentro da política que você declara. Modelos de fronteira com alto esforço permanecem dedicados a decisões de arquitetura, auditorias de segurança e investigação de bugs complexos, enquanto subagentes mecânicos rodam em modelos menores e mais ágeis, sem invalidar o cache de prompt da sessão principal.

| Capacidade | O que faz |
| --- | --- |
| **Compactação de contexto & saída** | Evita a degradação do contexto ao encurtar o ruído de comandos limpos, preservando logs de falha. |
| **Roteamento de modelo & esforço por subagente** | Aplica o tier do modelo e o nível de esforço de raciocínio na criação do subagente conforme a política. |
| **Relatórios locais & telemetria** | Mede tokens e contexto preservados sem enviar prompts, código ou caminhos para fora da máquina. |

[Quando usar o Downshift](../guides/when-to-use.md) · [Arquitetura](../design/architecture.md)

Selecione um diagrama para vê-lo em tamanho maior. Os rótulos das artes estão em inglês; o texto abaixo explica cada fluxo em português.

## Onde o Downshift atua

[![Um harness de programação acessa o Downshift por caminhos separados de roteamento de subagentes e execução de comandos; as observações dependem do harness](../brand/readme-media/operating-surfaces.svg)](../brand/readme-media/operating-surfaces.png)

O Downshift participa em dois pontos distintos: a criação de um subagente pode ser verificada contra a política e os modelos conhecidos da sessão, e a saída de comandos suportados pode ser compactada antes de chegar ao agente. Integração e observação variam entre harnesses; a execução direta do compactor está disponível quando não há encapsulamento automático.

## Reduza a saída dos comandos e mantenha os detalhes

[![Um comando de desenvolvimento gera uma saída completa; o Downshift mostra uma versão menor ao agente, enquanto a saída bruta elegível pode ser armazenada localmente com CCR e recuperada depois](../brand/readme-media/compaction.svg)](../brand/readme-media/compaction.png)

Um comando suportado é executado normalmente. O Downshift mostra uma versão mais curta ao agente, preservando o código de saída e os detalhes de falhas. Com o **Command Context Recovery (CCR)** ativado, a saída bruta elegível pode ser armazenada localmente e recuperada depois; linhas repetidas podem ser agrupadas no resultado exibido. O encapsulamento depende do harness, e a execução direta do compactor está disponível. A redução da saída varia conforme o comando e o formato.

```bash
downshift compact -- go test ./...
```

A saída limpa é compactada enquanto falhas e códigos de saída são preservados.

[![As famílias reconhecidas incluem testes, builds e verificações, Git e infraestrutura; o comportamento depende do comando e do formato de saída](../brand/readme-media/command-families.svg)](../brand/readme-media/command-families.png)

O Downshift reconhece formas específicas de comandos de teste, build, lint, Git e infraestrutura. Formatos reconhecidos podem ser compactados ou repassados conforme a família e a saída. Formatos de saída desconhecidos permanecem brutos; um comando fora da lista permitida é recusado pelo encapsulador e pode ser executado diretamente. Os detalhes de falhas e o código de saída são preservados. Consulte a referência de ferramentas para conhecer as formas exatas.

[Compactação](../features/compaction.md) · [Ferramentas suportadas](../reference/supported-tools.md) · [CCR](../guides/ccr.md)

## Roteie subagentes dentro da sua política

[![O orquestrador delega o trabalho; o Downshift verifica a política, os modelos conhecidos da sessão e as capacidades do harness antes de aplicar uma escolha ou manter o modelo solicitado](../brand/readme-media/routing.svg)](../brand/readme-media/routing.png)

O orquestrador escolhe o trabalho a delegar. O Downshift aplica decisões de modelo na criação do subagente, sujeito à sua política e às informações de modelos conhecidas para aquele harness e sessão. Quando não consegue estabelecer uma escolha segura, mantém o modelo solicitado.

A profundidade da integração varia. Uma reescrita emitida pelo Downshift, por si só, não comprova que o harness a aplicou. O código de roteamento do Downshift não faz uma chamada adicional a LLM. A escolha se aplica à criação do subagente; o Downshift não altera o modelo da sessão principal no meio do turno.

[Roteamento de modelos](../features/routing.md) · [Política](../guides/policy.md) · [Modelos da sessão](../reference/session-models.md)

## Veja seus agentes em ação

![Ilustração conceitual inspirada no painel Downshift Agents: um agente líder coordena agentes especialistas](../brand/readme-media/agents.png)

O Downshift inclui perfis de agentes e playbooks de orquestração para pesquisa, implementação, revisão, segurança, documentação e outras atividades. Um líder coordena o trabalho independente e reúne os resultados.

<a id="5-agent-orchestration-context-triggers"></a>
### Orquestração de agentes (gatilhos contextuais)

No chat do seu harness de programação, descreva sua tarefa e digite:
- `downshift full-power` (alias `potência total` / `potencia-total`): ativa os especialistas necessários respeitando o teto de capacidade da sessão.
- `downshift shift <squad>`: ativa um domínio específico (`core`, `quality`, `security`, `leadership`).

<p align="center"><img src="../brand/readme-media/agents-panel-illustration.png" width="420" alt="Ilustração sanitizada baseada em um painel Downshift Agents"></p>

*Ilustração sanitizada baseada em um screenshot de uma sessão Claude Code. Nomes de tarefas, modelos e métricas da sessão foram ocultados. Esta é uma prévia visual editada, não um benchmark ou screenshot sem alterações.*

[Guia de agentes](../guides/agents.md) · [Catálogo de agentes](../agents/README.md)

## Funciona com seu harness de programação

O Downshift foi criado com Claude Code e oferece suporte a outros harnesses em diferentes estágios de verificação.

| Harness | Status | Evidência de roteamento |
| --- | --- | --- |
| Claude Code | Concluído | Reescrita observada em um subagente real. |
| Codex | Funcional | Reescrita emitida; aplicação inferida, sem observação direta. |
| OpenCode | Beta | Roteamento e uso do subagente observados no OpenCode 2.2.5. |
| Cursor | Experimental | Reescrita emitida; aplicação pelo executor não confirmada. |
| Antigravity | Experimental | Um subagente observado em uma versão. |
| KiroCrew | Experimental | Modo de política, sem reescrita de modelo no lugar. |

Os modelos disponíveis para um harness ou sessão não são considerados disponíveis automaticamente em outro. Consulte a [matriz completa de harnesses](../reference/harness-matrix.md) para detalhes de integração, evidências e limites.

## O que as evidências mostram

| Medição | Resultado | Escopo |
| --- | --- | --- |
| Compactação de saída sem perda | **65,2% menos bytes** | 59 fixtures em um corpus sintético. |
| Compactação de saída com CCR | **72,0% menos bytes** | Corpus sintético de 107 fixtures; repetições exibidas podem ser agrupadas e a saída bruta pode ser recuperada. |
| Roteamento de subagentes | **24,1% menos tokens por execução aceita** | Estudo controlado com 46 execuções no Claude Code para tarefas mecânicas, realizado antes da decisão O8. |

São medições separadas, com unidades e cargas de trabalho diferentes. Os totais do corpus sintético incluem apenas fixtures processadas pelo compactor e excluem fixtures preservadas. O resultado de roteamento é evidência histórica; ele não estabelece o efeito da implementação após O8. A economia no uso cotidiano sem restrições ainda não foi estabelecida. Estimativas de tokens baseadas em bytes não são uma fatura nem uma medida dos limites do plano.

[Benchmark de compactação](../reference/bench.md) · [Estudo de roteamento](../evidence/four-arm-mechanical-2026-10-21.md) · [Guia de evidências](../guides/evidence.md)

## Feito para inspeção e controle

| Comando | Descrição e Finalidade |
|---|---|
| `downshift setup [--dry-run]` | Configura hooks nos harnesses detectados com prévia |
| `downshift doctor` | Diagnóstico de integridade do ambiente com correções automáticas |
| `downshift compact -- <cmd>` | Executa comando permitido com compactação de saída limpa |
| `downshift retrieve <id>` | Recupera saída bruta mantida no buffer seguro do CCR |
| `downshift session [--why]` | Exibe a árvore de subagentes da sessão e a justificativa das decisões |
| `downshift stats` | Resumo de decisões de roteamento e estimativa de tokens economizados |
| `downshift report` | Relatório consolidado de tokens direcionados por tarefa concluída |
| `downshift policy check` | Valida sintaxe e semântica do `policy.json` |
| `downshift quota` | Diagnóstico de cotas e janelas de consumo dos provedores |
| `downshift mcp serve` | Inicia servidor MCP local via stdio (13 ferramentas nativas) |

- Hooks falham abertos: erros ou timeouts mantêm a criação do subagente solicitada sem alteração.
- O roteamento é local e não faz uma chamada adicional a LLM. A telemetria registra contagens, hashes e tempos, sem registrar prompts.
- O CCR pode armazenar saídas brutas de comandos localmente. A sanitização cobre formatos comuns de credenciais, mas segredos em formatos arbitrários podem permanecer; consulte as [limitações de saída bruta](../reference/known-limitations.md#raw-output-kept-by-ccr) e a configuração antes de usá-lo com saídas sensíveis.
- Workers com escrita exigem ativação explícita e usam worktrees separados. O Downshift não mescla suas branches nem oferece uma etapa de aprovação para deploys e outras ações de risco.

[Segurança](../features/safety.md) · [Telemetria](../features/telemetry.md) · [Limitações conhecidas](../reference/known-limitations.md)

## Documentação e comunidade

[Início rápido](../guides/quickstart.md) · [Referência de CLI](../reference/cli.md) · [Arquitetura](../design/architecture.md) · [English](../../README.md)

Encontrou um problema ou quer contribuir? Leia o [guia de contribuição](../../CONTRIBUTING.md) ou [abra uma issue](https://github.com/tiagovilasboas/downshift-ai/issues).

## Licença

[Apache License 2.0](../../LICENSE). Downshift por [Tiago Vilas Boas](https://github.com/tiagovilasboas).
