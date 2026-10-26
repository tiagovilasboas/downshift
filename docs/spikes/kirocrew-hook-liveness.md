<!-- Copyright (c) 2026 Tiago Vilas Boas. SPDX-License-Identifier: Apache-2.0 -->

# Spike: o KiroCrew chama o hook do Downshift num spawn de verdade?

Downshift by Tiago Vilas Boas · https://github.com/tiagovilasboas/downshift

**Data:** 2026-10-26 · **Status:** aberto · **Dono do resultado:** Downshift (hook, doctor, setup).

## Pergunta

Quando o agente do KiroCrew dispara um subagente (`spawn_run`) a partir do chat, o `preToolUse` do Downshift é
chamado? Se for, o Downshift grava a decisão. Se não for, o roteamento não existe nesse caminho, e o usuário
não tem como saber.

## O que já sabemos

| Fato | Evidência |
|---|---|
| O hook funciona isolado: um payload de spawn em opus é bloqueado com recomendação de haiku | Rodado à mão com o binário instalado e com a bancada `harnesssim` |
| O wrapper e os specs dos agentes apontam para um script que existe | `~/.downshift/bin/kirocrew-pre-tool-use.sh` e `~/.kiro/agents/*.json` |
| Um spawn do chat às 01:37 rodou o subagente em `claude-opus-4.5` e escreveu o arquivo pedido | `~/.kiro/crew/subagents/9b566318912a531b/state.json` |
| Esse mesmo spawn **não deixou nenhum evento** do Downshift | Sem o session id em `events.jsonl`, `exec-events.jsonl` e `agents.jsonl` |
| O log do gateway não mostra erro de hook nesse horário | `gateway-launch.log` |
| O último evento de roteamento do KiroCrew é de 23/10 | `events.jsonl` |

## Hipóteses

| # | Hipótese | Como refuta |
|---|---|---|
| H1 | O `preToolUse` não é chamado para `spawn_run` disparado pelo chat | Batimento presente para outros tools, ausente para `spawn_run` |
| H2 | O hook é chamado, mas o resultado não é gravado (falha silenciosa, fail open) | Batimento presente com `tool_name=spawn_run` e nenhum evento de decisão |
| H3 | O hook é chamado, mas por um wrapper com caminho antigo que não existe | O doctor reprova o comando; o gateway loga "command not found" |
| H4 | O sandbox do gateway impede o hook de gravar no estado do Downshift | Batimento ausente com sandbox em `auto`, presente com `off` |

## Método

O Downshift passa a registrar um **batimento** em toda chamada de hook, em qualquer harness e qualquer tool:
só horário, harness e nome do tool. Nenhum texto de prompt, nenhum caminho, nenhum conteúdo de sessão. O
`downshift doctor` mostra a idade do último batimento por harness e avisa quando o harness está ligado e nunca foi
chamado nas últimas 24 h.

Com o batimento, um único spawn de teste no KiroCrew separa as hipóteses:

1. O batimento aparece para `spawn_run` → H1 refutada; se não houver evento de decisão, é H2.
2. O batimento não aparece → H1 ou H4. Repetir com o sandbox em `auto` e em `off`.
3. O doctor reprova o comando do hook → H3, e `downshift setup --yes` corrige.

## Critério de pronto

- `downshift doctor` mostra, para o KiroCrew: o comando existe, a prova viva passa e há um batimento recente.
- Um spawn real no KiroCrew produz um batimento e um evento com `requested_model` e `final_model`, e o
  `resolved_model` do `state.json` do subagente bate com o `final_model`.

## Riscos

- **Loop de login.** Qualquer teste que rode o kiro-cli dentro de um HOME sem sessão abre a tela de login. Os
  testes deste trabalho usam fakes e `DOWNSHIFT_DISCOVERY=off`. Nada chama o kiro-cli de verdade.
- **Sandbox em `off`.** O config do KiroCrew está com `agent.sandbox = off` desde 2026-10-26, para testar a
  hipótese H4. O backup do valor anterior (`auto`) está guardado. Decidir se volta para `auto` depois do teste.
- **O agente do KiroCrew recusa spawn trivial** e pede 3 GB livres de RAM para iniciar um subagente. O teste
  precisa de uma tarefa que justifique o subagente.

## Fora de escopo

- Reescrever modelo ou comando no KiroCrew: o hook só permite ou bloqueia (exit 0 ou 2).
- Contagem de tokens: o provedor ACP do KiroCrew reporta zero.
