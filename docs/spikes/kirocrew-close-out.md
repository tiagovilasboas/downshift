<!-- Copyright (c) 2026 Tiago Vilas Boas. SPDX-License-Identifier: Apache-2.0 -->

# KiroCrew: fechamento do spike e do REA

Downshift by Tiago Vilas Boas · https://github.com/tiagovilasboas/downshift

**Data:** 2026-10-26 · **Escopo:** o que o Downshift faz no KiroCrew hoje, o que foi
provado, o que não foi, e a nota honesta.

Este documento fecha o spike de [KiroCrew shell policy](kirocrew-shell-policy.md) e
resume o trabalho de reverse engineering (REA). O REA completo fica no repositório
privado do mantenedor; a versão pública é este resumo.

## Nota: 5/10

- **Detecção (7/10):** o harness é identificado pela variável `KIRO_SESSION_ID`. O modelo
  da sessão é inferido dos arquivos do KiroCrew. Após a correção do leitor de log
  (RV2-1, PR #568), a inferência devolve o modelo certo nos logs reais medidos.
- **Roteamento de modelo (0/10):** o hook do KiroCrew só permite ou bloqueia a chamada
  (exit 0 ou 2). Não há canal de `updated_input`, então o Downshift não troca o modelo do
  subagente nesse harness.
- **Compactação (5/10):** funciona por comando explícito (`downshift compact -- <cmd>`).
  Não há reescrita automática de comando.
- **Tokens (0/10 por enquanto):** o provedor ACP zera os campos de uso. A economia só
  pode ser estimada por proxy (`context_used`), e isso é `partial`.

A nota fica em 5 porque a parte que o Downshift vende, roteamento, não existe nesse
harness. Detecção e compactação funcionam, mas sem prova ao vivo.

## Capacidades hoje

| Capacidade | Estado | Como |
|---|---|---|
| Detecção de harness | Feito | `SessionEnv: "KIRO_SESSION_ID"` em `KiroCrewCaps` |
| Detecção de modelo | Feito, sem prova ao vivo | `discovery.InferKiroCrewModel`, lendo `subagents/<id>/state.json` e o log do pai |
| Modelo nos exec-events | Feito | `execwrap.Event.Model`, preenchido por `envModel` |
| Troca de modelo (rewrite) | Não suportado | Hook só permite ou bloqueia |
| Troca de esforço (effort) | Não suportado | Mesmo motivo |
| Compactação de shell | Manual | Agente chama `downshift compact --` |
| Uso de tokens | Bloqueado | Campos zerados no provedor ACP |
| Economia por contexto | Parcial | Proxy `context_used`, sem tokens reais |

## O que o REA descobriu

| ID | Descoberta | Consequência |
|---|---|---|
| F1 | `KIRO_SESSION_ID` está no ambiente | Identifica o harness e a sessão |
| F2 | O modelo não está no ambiente | Inferência pelos arquivos da sessão |
| F3 | O exec-event pode carregar o modelo | Campo `model` gravado, com a mesma inferência |
| F4 | Os campos de uso de tokens existem, mas vêm zerados | Economia em tokens não é medível |
| F5 | O hook nativo só tem exit 0 ou 2, sem `updated_input` | Sem reescrita de modelo nem de comando |
| F6 | O modelo de uma sessão principal fica em `~/.kiro/sessions/cli/<id>.json` | Segundo caminho de inferência |
| F7 | A causa dos zeros é o provedor ACP, que não reporta tokens | Limitação do produto, não do Downshift |
| F8 | O KiroCrew é Electron com backend Python | Explica onde ficam os arquivos de sessão |

## O que foi medido

- Os logs reais de `~/.kiro/crew/sessions` (7 a 18 MB cada) devolvem o modelo do
  último turno do assistente.
- Antes da correção, um log devolvia um modelo antigo e outros dois devolviam
  `unknown`. Depois, os três valores batem com o último turno do arquivo.
- A suíte `harnesssim` do KiroCrew passa.

Números de desempenho e as remoções de código ficam nos PRs de fechamento e não são
repetidos aqui.

## O que não foi provado

- **Prova ao vivo (P3):** nenhuma sessão real do kiro-cli rodou com este build para
  confirmar que o modelo detectado é o que o KiroCrew usou. Essa prova depende de uma
  sessão do dono.
- **Economia em tokens:** sem campos de uso, não há número de tokens economizados.
- **Compactação automática:** não existe no KiroCrew. Não há como provar o ganho em uso
  real sem o comando explícito.

## Pendências

1. Prova ao vivo: rodar um spawn real no kiro-cli com este build e comparar o modelo
   detectado com o modelo da sessão. Dono: maintainer.
2. Release: as seções `[0.1.4]` e `[0.2.0]` do CHANGELOG ainda não existem. Decisão do
   dono sobre editar a release publicada ou abrir `0.2.1` (RV2-3).
3. README: ainda usa o nome antigo `compactor` em uma seção. Corrigir junto com a release.
