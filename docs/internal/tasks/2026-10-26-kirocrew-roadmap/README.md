<!-- Copyright (c) 2026 Tiago Vilas Boas. SPDX-License-Identifier: Apache-2.0 -->

# Roadmap: liveness do hook KiroCrew no Downshift

Downshift by Tiago Vilas Boas · https://github.com/tiagovilasboas/downshift

**Data:** 2026-10-26 · **Objetivo:** provar se o hook é chamado em um **spawn_run** real e tornar a ausência de chamadas diagnosticável.

## Estado atual

| Item | Estado | Evidência |
|---|---|---|
| Descoberta de modelos sem listas fixas e proteção contra execução do **kiro-cli** sem login | Feito | PRs #570, #575 e #578 no **main** |
| Setup de hooks KiroCrew e wrapper com fallback para o binário no PATH | Feito | PR #576, **setup_targets.go**, **setup_wrapper.go** e testes |
| Doctor verifica wiring e eventos de roteamento | Feito | **doctor_hooks.go** |
| O hook registra toda chamada, mesmo sem decisão de roteamento | Aberto | O runner atual só grava evento quando produz uma decisão |
| Chamada real de **spawn_run** e modelo resolvido conferidos de ponta a ponta | Aberto | Um spawn anterior concluiu sem evento do Downshift; a causa continua ambígua |

O código confirma a configuração esperada, mas ainda não prova que o KiroCrew a executa no caminho iniciado pelo chat. O spike detalha as hipóteses e o experimento: [kirocrew-hook-liveness](../../../spikes/kirocrew-hook-liveness.md).

## Sequência de entrega

| Ordem | Incremento | SP | Dependência | Paralelismo | PR |
|---:|---|---:|---|---|---|
| 1 | Publicar spike, roadmap e tarefas com o contrato e os critérios de aceite | 2 | — | Pesquisa de código | Docs, antes da implementação |
| 2 | Dividir os arquivos de hook e doctor que excedem 500 linhas, sem mudar comportamento | 4 | 1 | Plano de testes e revisão AppSec | Refactor independente |
| 3 | Adicionar batimento limitado ao runner compartilhado e estado recente de **spawn_run** no doctor | 5 | 2 | QA e AppSec após o formato do registro estar estável | Código e testes juntos |
| 4 | Revisão AppSec de privacidade, limites, permissões e fail-open | 2 | Contrato de 3 | Paralelo com implementação e QA | Revisão da PR de runtime |
| 5 | Verificação QA de casos válidos, ausentes, inválidos e falhas de escrita | 3 | Contrato de 3 | Paralelo com implementação e AppSec | Evidência na PR de runtime |
| 6 | Atualizar o guia KiroCrew para explicar configuração, batimento e evento de roteamento | 1 | 3 | — | Docs, depois do comportamento |
| 7 | Rodar um único spawn real e comparar batimento, decisão e **resolved_model** | 2 | 3 e 5 | — | Evidência operacional |
| 8 | Atualizar spike e roadmap com o resultado real | 1 | 7 | — | Docs de fechamento |

As tarefas detalham arquivos, donos e aceites em [tasks.md](tasks.md). Cada incremento tem no máximo 5 pontos. O teste real vem depois dos checks automatizados e exige que o KiroCrew consiga iniciar um subagente; a exigência conhecida é pelo menos 3 GB livres.

## Decisões de escopo

- A observabilidade fica no runner compartilhado para não criar uma regra exclusiva do KiroCrew.
- O armazenamento é limitado e guarda somente horário UTC, harness e nome de ferramenta validado. A última chamada de **spawn_run** é consultada separadamente.
- Nenhuma falha de telemetria pode alterar a decisão ou impedir a chamada do harness.
- O teste mantém a descoberta desligada e não executa a descoberta de modelos.
- Esta entrega não muda sandbox, credenciais, agenda, publicação ou release. Comparar modos de sandbox exige decisão específica.

## Critério de pronto

1. **downshift doctor** distingue configuração, batimento e decisão de roteamento; mostra a idade de **spawn_run** especificamente.
2. Testes e **make check** passam no incremento de runtime.
3. O spawn real produz um batimento e um evento de roteamento, e o modelo resolvido pelo KiroCrew corresponde ao modelo final registrado.
4. Se o teste real não iniciar, o resultado permanece aberto e aponta o bloqueio observado.

## Fora deste roadmap

Compatibilidade dos modelos 5-5, aliases entre IDs de modelos, reporte de tokens, loop de login, unificação de instalações locais e decisões de release seguem como trabalho separado.
