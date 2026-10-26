<!-- Copyright (c) 2026 Tiago Vilas Boas. SPDX-License-Identifier: Apache-2.0 -->

# Roadmap: KiroCrew funcionando de ponta a ponta no Downshift

Downshift by Tiago Vilas Boas · https://github.com/tiagovilasboas/downshift

**Data:** 2026-10-26 · **Objetivo:** um usuário que só tem o KiroCrew instalado consegue usar o Downshift com
roteamento real, sem nenhum caminho desta máquina.

Legenda: **FEITO**, **ABERTO**, **DECISÃO** (depende do dono), **PROVA** (depende de uma sessão ao vivo).

## Estado atual (o que já está no `main` ou na máquina)

| Item | Estado | Evidência |
|---|---|---|
| Descoberta de modelos sem listas fixas, para todos os harnesses | FEITO | PR #570 mergeado |
| Checagem de login antes de usar a cópia do kiro-cli embutida no app | FEITO | PR #575 mergeado (`6f52d5d2`) |
| Close-out do KiroCrew: leitor de log do fim, métricas sem uso removidas, doc de status | FEITO | PR #569 mergeado |
| Spike: dependência do KiroCrew em relação ao kiro-cli | FEITO | PR #574 mergeado |
| Downshift reinstalado a partir do `main` em `~/.local/bin/downshift` | FEITO | `downshift version` = `v0.2.0-6-g6f52d5d2` |
| Cache de descoberta do KiroCrew gravado (4 modelos usáveis) | FEITO | `downshift doctor`: `session list: discovered (4 models)` |
| kiro-cli 2.34.0 instalado; doctor do KiroCrew passa (`kiro-cli ✅`, `kiro auth ✅`) | FEITO | `kirocrew doctor` |
| Mochi desativado no app | FEITO | Biblioteca mostra "Desativado"; `/mochi` mostra "Mochi está dormindo" |
| Login do painel do KiroCrew restaurado | FEITO | Feito pelo dono no chat, em `localhost:5476` |
| Abas de login e item de login do KiroCrew | FEITO | Abas fechadas; KiroCrew removido dos itens de login |
| Spawn de teste via chat | PARCIAL | Subagente rodou e escreveu o arquivo; `resolved_model=claude-opus-4.5`; **nenhum evento do Downshift** |
| Testes não disparam refresh real da descoberta (stub em `TestMain`) | FEITO | `cmd/downshift/main_test.go` no `main` |
| Resolvedor do kiro-cli: override, cópia embutida só se logada, PATH | FEITO | `internal/discovery/kiro_bin.go` no `main` |
| Causa do loop de abas de login identificada | FEITO | Seção "Loop de login" abaixo |

## Pendências

### A. Provar o roteamento ao vivo (bloqueante)

1. **ABERTO / PROVA:** o hook do KiroCrew não deixou registro para o spawn de 01:37 (`session 1f06ac5d…`).
   Não há evento em `events.jsonl`, `exec-events.jsonl` nem `agents.jsonl`, e o log do gateway não mostra
   erro de hook nesse horário. Hipóteses: o `preToolUse` não é chamado para `spawn_run` disparado pelo chat,
   ou o hook roda e não grava.
2. **DECISÃO:** autorizar um experimento que grava um arquivo a cada chamada do hook (sinal externo, sem
   depender do log). Mexe na configuração do KiroCrew, por isso precisa de ok.
3. **PROVA:** com o sinal confirmado, rodar um spawn com um subagente que precise de roteamento
   (opus → haiku) e conferir o evento e o `resolved_model`.

### B. O Downshift se responsabiliza pela instalação do hook (sem caminhos da máquina)

Problema: o wrapper grava o caminho absoluto do binário que rodou o `setup` (`os.Executable()` em
`cmd/downshift/setup.go`). Quem move ou reinstala o binário fica com um hook quebrado. Um usuário que só tem
o KiroCrew não tem como saber disso.

4. **ABERTO:** o wrapper resolve o `downshift` na execução: primeiro o caminho gravado, se existir; depois
   `command -v downshift`; senão, mensagem clara e exit 0 (fail open).
5. **ABERTO:** `downshift doctor` reprova hook que aponta para comando inexistente ou não executável
   (o caso do caminho antigo `harness-downshift/scripts/...` no log do KiroCrew), com a correção
   `downshift setup --yes`.
6. **ABERTO:** `downshift doctor` faz uma prova viva: roda o wrapper com um payload de spawn de fixture, em
   diretório temporário, e espera a linha `downshift:` com exit 0 ou 2.
7. **ABERTO:** testes no nível compartilhado, sem tocar em `~/.downshift`, `~/.kiro` nem no kiro-cli real.

Um agente chegou a começar esse trabalho, mas foi parado antes de commitar. Nada foi aberto como PR.

### C. Um binário só

8. **ABERTO:** o `make install` grava em `~/go/bin`, e o hook antes apontava para `~/.local/bin`. O setup
   deve usar um local único e informar qual é, para não haver duas cópias divergindo.

### D. Descoberta e modelos

9. **ABERTO:** o modelo `claude-haiku-5-5` e o `claude-sonnet-5-5` não entram como candidatos mesmo
   com o catálogo tendo as linhas `claude-code`. Com `claude-haiku-4-5` e `claude-sonnet-4-5` na lista, o
   roteamento funciona. Investigar se a lista do KiroCrew (`kiro-cli chat --list-models`, versão 2.34.0)
   inclui os ids `5-5`. O `provider_models.json` do KiroCrew é um cache de 23/10 e não tem esses ids.
10. **ABERTO:** o `resolved_model` do subagente reporta `claude-opus-4.5` com o id nativo. O Downshift deve
    comparar o modelo pedido e o resolvido nesse formato, senão um downshift real parece "não aplicado".

### E. Documentação e release

11. **DECISÃO:** seções `[0.1.4]` e `[0.2.0]` no CHANGELOG, status do README (ainda diz Beta v0.1.3) e
    notas da v0.2.0 (editar a release publicada ou abrir `0.2.1`). Isso é do dono.
12. **ABERTO:** `docs/pt/README.md` e as matrizes podem estar desatualizados em relação ao KiroCrew.
13. **ABERTO:** o REA do labs (`reverse/kirocrew/CURATION-kirocrew-only.md`) está em branch, sem PR. Decidir
    se entra, e incluir as fontes oficiais de instalação.

### F. Limites do produto, sem correção no Downshift

14. **Compactação** no KiroCrew é manual (`downshift compact -- <cmd>`), porque o hook não reescreve comando.
15. **Tokens** não são reportados pelo provedor ACP do KiroCrew, então a economia só é estimada por proxy.
16. **Subagentes** do KiroCrew pedem 3 GB livres de RAM, e o agente recusa spawn trivial. Documentar como
    comportamento do produto.

### G. Higiene

17. **DECISÃO:** token de sessão do gateway exposto no histórico do chat. Gerar um novo com `kirocrew token`.
18. **ABERTO:** PRs do Dependabot #571, #572 e #573. Mergear quando o CI estiver verde.
19. **ABERTO:** teste de tempo `internal/hookctx` (`TestRunCommand_BackgroundChildDoesNotHang`) falha às
    vezes sob carga. Não é regressão, mas precisa de folga de tempo ou de um teste sem relógio.

## Ordem sugerida

1. A (decisão 2) → prova ao vivo. É o que fecha o KiroCrew.
2. B (itens 4 a 7). Depende de A para validar a prova viva.
3. D (item 9), para os ids `5-5`.
4. C (item 8), depois de B.
5. E e G, conforme as decisões do dono.

## Como validar que está pronto

- `downshift doctor` sem falhas nem avisos de hook para um usuário sem nenhum caminho desta máquina.
- Um spawn real no KiroCrew gera evento no Downshift com o `requested_model`, o `final_model` e o
  `resolved_model` do subagente.
- `make check` passa no `main`.

## Loop de login (causa identificada)

- O painel do KiroCrew e o Mochi, quando aberto, consultam `/api/sessions/usage` a cada 30 s.
- Cada consulta roda `kiro-cli chat --no-interactive ... /usage`. Sem sessão válida no contexto do gateway,
  o kiro-cli abre a página de login. Cada tentativa abre uma aba (até 14 abertas nesta máquina).
- A trava que pausa essas tentativas fica na memória do gateway. Cada reinício do gateway zera a trava, e o
  gateway reiniciou dezenas de vezes.
- Com o login do painel restaurado, o loop parou. Falta confirmar se o kiro-cli do gateway continua logado
  após o próximo reinício.
- **Dono do ajuste:** KiroCrew (a sonda de usage). No Downshift, nenhuma chamada de hook dispara essa sonda;
  o refresh da descoberta roda o kiro-cli só pelo `models discover`, e só com a checagem de login (#575).

## Artefatos de teste a limpar (decisão do dono)

- `/tmp/downshift-kirocrew-task.txt`, criado pelo subagente do teste. Serve de evidência do spawn; apagar depois.
- Sessões de teste no KiroCrew: "Rename Variable userId Task", "KiroCrew subagent spawn check" e
  "Subagent spawn file write check". Ficam no histórico do chat; apagar só com ok.
- Subagente `9b566318912a531b` aparece como "running" no painel, apesar de já ter terminado.

## Aprovações do KiroCrew (comportamento do produto)

- Cada subagente pede "Permitir uma vez" ou "Aprovar" para escrever arquivo. Para um teste automatizado,
  isso exige o dono presente. Não é falha do Downshift.

## Onde paramos (2026-10-26, 04:40 BRT)

1. **Mergeado e no `main`:** #569, #570, #574, #575. Downshift reinstalado (`6f52d5d2`).
2. **Parado antes de commitar:** a correção do hook (item 4 a 7). O agente que começou foi parado, e o worktree
   `wt-hook` ficou parcial. Nada foi aberto como PR.
3. **Aguardando o dono:** o ok para o experimento do item 2 (sinal de chamada do hook). Sem isso, não há como
   confirmar se o `preToolUse` roda para `spawn_run` do chat.
4. **Este roadmap:** commitado em `docs/kirocrew-roadmap` (`ab1178f5`), sem push.

Próxima ação quando você liberar: experimento do sinal (item 2), depois o spawn com o subagente de
roteamento (item 3).
