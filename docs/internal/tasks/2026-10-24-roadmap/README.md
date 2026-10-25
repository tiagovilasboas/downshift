<!-- Copyright (c) 2026 Tiago Vilas Boas. SPDX-License-Identifier: Apache-2.0 -->

# Roadmap (atualizado 2026-10-24)

Downshift by Tiago Vilas Boas · https://github.com/tiagovilasboas/downshift

Estado real do que está no `main` e do que falta, com cada tarefa aberta detalhada: arquivos,
critério de pronto, dependência e pontos (Fibonacci, até 5). Os planos de origem ficam ao lado
de cada frente. Estado conferido no GitHub e no código do `main` em 2026-10-24.

Legenda: **FEITO** (no main), **ABERTO** (fazer), **DECISÃO** (depende do dono), **PROVA** (depende de
uma sessão ao vivo com o dono).

## 1. downshift-progress ([plano](../2026-10-24-downshift-progress/README.md))

| ID | Tarefa | Estado | Arquivos | Pronto quando | Depende | Pts |
|---|---|---|---|---|---|--:|
| DP-0 | Estrutura, cliente, formato/i18n/cores, `progress`/`step`, roster, painel, barra, campos do painel | FEITO (#528–#541) | `mods/downshift-progress/**` | — | — | — |
| DP-1 | Corrigir formato de resultado das ferramentas (string ou `deny`) | ABERTO (#542, CI) | `hooks/features/flow/tools.ts`, `flow.test.ts` | #542 mesclado; `progress` ao vivo sem erro de formato (provado em 2026-10-24) | CI do #524 | 1 |
| DP-2 | Prova lado a lado com o savvy-progress | PROVA | `docs/evidence/downshift-progress-parity-<data>.md` | tiles batem com `downshift report --json` na mesma janela, em sessão ao vivo | DP-1 | 3 |
| DP-3 | Corte: `setup` instala o downshift-progress; entrada no marketplace; `doctor` reconhece | ABERTO + DECISÃO (entrar no marketplace agora ou só no corte) | `cmd/downshift/setup_targets.go` (`Mods`), `setup_plugin_test.go`, `.claude-plugin/marketplace.json`, `mods/downshift-progress/.claude-plugin/plugin.json` | `setup --yes` num HOME limpo instala `downshift-progress@downshift` e não o savvy-progress (teste) | DP-2 | 3 |
| DP-4 | Apagar o savvy-progress (mod, NOTICE, README) | ABERTO, depois de uma release | `mods/savvy-progress/**`, `NOTICE`, `docs/guides/*` | nenhuma referência viva; testes e `make check` verdes | DP-3 + release | 2 |

## 2. Contabilidade ([plano](../2026-10-21-accounting-v1/README.md))

| ID | Tarefa | Estado | Arquivos | Pronto quando | Depende | Pts |
|---|---|---|---|---|---|--:|
| AC-0 | Conta única (`Compute`), paridade, regras de camada | FEITO (#489, #500, #508, #520) | `internal/accounting/**` | — | — | — |
| AC-1 | Apagar agregações sem uso fora do pacote | ABERTO (confirmado: 0 chamadas externas) | `internal/accounting/global.go` (293 linhas), `harness.go` (108), `savings.go` (109) e as entradas da allowlist do `archtest` | `go vet ./...` e testes verdes; `git grep` sem chamadas; allowlist sem essas entradas | — | 3 |
| AC-2 | Dividir `internal/telemetry/telemetry.go` (916 linhas) | ABERTO | `telemetry.go` → tipos de evento, escrita do log, leitura do log, vínculo de decisão | nenhum arquivo não-teste de `internal/telemetry` acima de 300 linhas; teste de paridade igual | AC-1 | 5 |
| AC-3 | `stats` vira alias de `report --days` (D6), `--export` vira alias deprecado | ABERTO (contrato de flags) | `cmd/downshift/stats*.go`, `report.go` | `stats --json` tem os mesmos números que `report --json` na mesma janela; aviso de deprecação | AC-2 | 5 |
| AC-4 | Teto de bytes visíveis por harness (W5): `saved_bytes` limitado ao que o harness mostra | ABERTO + DECISÃO (valor exato do Claude Code, hoje só relatos de comunidade ~30 mil caracteres) | `internal/core/capabilities.go` (campo novo), `internal/accounting` | tabela de teste com 30 KB e 2 MB; valor confirmado na doc antes de virar padrão | decisão do dono | 3 |
| AC-5 | USD secundário nas superfícies (decisão D1) | ABERTO, depois da paridade e do teto | `internal/accounting/display*` | USD sempre com o rótulo "estimativa a preço de lista", na mesma fonte do percentual | AC-4 | 3 |

## 3. Compactor ([plano](../2026-10-23-minimal-wins/README.md))

| ID | Tarefa | Estado | Arquivos | Pronto quando | Depende | Pts |
|---|---|---|---|---|---|--:|
| CP-0 | `go test` limpo e com falha, `2>&1`, `VAR=`, Jest no stderr, pnpm/yarn/php, docker, orientação dos harnesses, reescrita no Codex e no Cursor | FEITO (#491, #497, #499, #501 via #512, #503, #505, #506, #507, #498, #494) | `internal/compressor/**`, `internal/execwrap/**` | — | — | — |
| CP-1 | `gh run view --log`: manter só os passos com falha | ABERTO (#537, BEHIND→atualizado, CI) | `internal/compressor/gh_run*.go` + golden | #537 mesclado; golden com log real | — | 5 |
| CP-2 | Jest com falha: manter cada bloco `●` com o diff, cortar as linhas `PASS` | ABERTO | `internal/compressor/jest*.go` + golden de falha | golden: cada `●` byte a byte; saída limpa continua igual | — | 5 |
| CP-3 | Playwright com falha: manter cada teste falho com erro e link do trace, cortar `✓` | ABERTO | `internal/compressor/playwright*.go` + golden | golden: falhas preservadas; passos verdes cortados | CP-2 | 5 |
| CP-4 | PHPUnit com falha: manter falhas e erros, cortar os pontos de progresso | ABERTO | `internal/compressor/phpunit*.go` + golden | golden idem CP-3 | CP-2 | 3 |
| CP-5 | Jest `--coverage`: dobrar a tabela na linha `All files` | ABERTO | `internal/compressor/jest*.go` | golden da tabela | — | 2 |
| CP-6 | `npm run <script>`: resolver o script do `package.json` do diretório e reescrever quando for Jest/Vitest/Playwright | ABERTO (não confirmado no main) | `internal/execwrap/policy_frontend.go` (ler o JSON, sem executar) | testes de script permitido e não permitido | — | 3 |
| CP-7 | `pnpm build` / `next build`: manter avisos e erros, dobrar a tabela de assets | ABERTO | `internal/compressor/pnpm*.go` | golden | — | 3 |
| CP-8 | Leituras de `gh pr` e `git log` só quando a saída for tabela reconhecida | ABERTO | `internal/compressor/*` | golden | — | 3 |
| CP-9 | Não compactar saída pequena de git (C3) | ABERTO, depois de olhar os dados de CP-0 | `internal/execwrap/policy*.go` | motivo `small_output` aparece no log e não há reescrita inútil | dados | 3 |

## 4. Roteamento e harnesses

| ID | Tarefa | Estado | Arquivos | Pronto quando | Depende | Pts |
|---|---|---|---|---|---|--:|
| RT-0 | Tipo do agente como sinal (#496 mesclado), calibração (#492), KiroCrew (#485, #488, #504) | FEITO | — | — | — | — |
| RT-1 | Mensagem de respawn do KiroCrew sai do adaptador (K4) | ABERTO | `internal/adapters/kirocrew/kirocrew.go` → camada compartilhada | adaptador só parseia e emite; teste na camada compartilhada | — | 2 |
| RT-2 | Upshift acima de modelo pedido explicitamente (C6) | DECISÃO | `internal/adapters/*`, `docs/reference/session-models.md` | dono decide sim, não ou só documentar | dono | 3 |
| RT-3 | Descoberta automática de modelos por sessão, ao iniciar (TTL, troca de plano, Kiro Pro) | ABERTO, spike primeiro | `internal/discovery/*`, `docs/reference/session-discovery.md` | spike com fonte por harness; tarefa de implementação só depois | — | 3 (spike) |
| RT-4 | Prova ao vivo Codex (`/hooks`, `go test` comprimido) e Cursor (`Shell`) | PROVA | `docs/evidence/*` | evidência registrada na matriz | dono (~15 min) | 2 |
| RT-5 | `doctor` mostra por harness se a reescrita de shell está ligada e honrada | ABERTO | `cmd/downshift/doctor*.go` | linha por harness; teste | RT-4 | 2 |

## 5. UX dos comandos ([plano](../2026-10-23-claude-code-and-ux/README.md))

| ID | Tarefa | Estado | Arquivos | Pronto quando | Depende | Pts |
|---|---|---|---|---|---|--:|
| UX-0 | `--help` em todo comando, erro de desconhecido em uma linha, `compact` no `help`, `session` em palavras simples, `doctor` curto | FEITO (#509, #513, #514, #516) | — | — | — | — |
| UX-1 | `report` começa pela resposta (3 linhas), detalhe com `--detail` | ABERTO | `cmd/downshift/report.go` | primeiras 3 linhas são a resposta; teste de texto | AC-3 (mesma tela) | 3 |
| UX-2 | `--json` em `models list` e `feedback`; `--period` alias de `--days` | ABERTO | `cmd/downshift/models*.go`, `feedback*.go` | flags novas com teste; nenhuma flag antiga some | — | 3 |

## 6. CI, estabilidade e repos

| ID | Tarefa | Estado | Arquivos | Pronto quando | Depende | Pts |
|---|---|---|---|---|---|--:|
| CI-0 | Windows, `make check`, lefthook, `merge_group`, testes de tempo só no CI (#524), guarda no harness-core, `local-ci` | FEITO (#524 em 2026-10-24) | `Makefile`, `lefthook.yml`, `.github/workflows/ci.yml`, `checks/**` (harness-core) | — | — | — |
| CI-1 | Aplicar lefthook nos outros repos (harness-core, opendraw, voomp-e2e) | ABERTO | `lefthook.yml` por repo | `make hooks` roda e um commit sem teste passa/quebra como esperado | — | 2 cada |
| CI-2 | Cobrança do GitHub Actions nos repos privados | DECISÃO (dono) | — | jobs de repo privado deixam de morrer em 2 s | dono | — |
| CI-3 | Mod tests em CI (F2) | DECISÃO | `.github/workflows/ci.yml` (job novo ou gate local) | decisão registrada e aplicada | dono | 3 |
| CI-4 | `scripts/`/`docs` sem `t.Log` ou ferramenta de teste; sem outras pendências | ABERTO se houver | — | — | — | — |

## 7. Release v0.1.4 e site

| ID | Tarefa | Estado | Arquivos | Pronto quando | Depende | Pts |
|---|---|---|---|---|---|--:|
| RL-1 | CHANGELOG `[Unreleased]` com tudo que entrou desde a v0.1.3 | ABERTO | `CHANGELOG.md` | uma linha por PR relevante | DP, CP, AC, UX | 2 |
| RL-2 | README: status beta v0.1.1 → v0.1.4; seção de Compactor e roteamento | ABERTO | `README.md`, `docs/pt/README.md` | sem "v0.1.1" no status | RL-1 | 1 |
| RL-3 | `harness-matrix.md` e `capability-matrix.md` com as linhas de hoje | ABERTO (parcial: H1, H2 atualizaram) | `docs/reference/*.md` | cada harness com estado e prova | RT-4 | 2 |
| RL-4 | Tag `v0.1.4-beta` | DECISÃO (aprovação explícita) | goreleaser | release publicada | RL-1..3, CI verde | 1 |
| RL-5 | PR no `downshift-site` (repo privado, Vercel): tile novo, famílias do Compactor, canal Codex/Cursor só se provado | ABERTO | repo `downshift-site` | deploy com números da conta nova; sem o "53%" antigo | RL-4 | 3 |

## Decisões que só o dono toma

1. **RT-2:** upshift acima de modelo pedido explicitamente.
2. **AC-4:** o teto de bytes visíveis por harness (valor do Claude Code).
3. **DP-3:** entrada no marketplace agora ou só no corte.
4. **CI-2:** cobrança do Actions (sem isso, repo privado não roda CI).
5. **CI-3:** mod tests em CI.
6. **RL-4:** tag da release.
7. **OpenRouter (D-1/D-2, parts 4–6):** por último — não antecipar antes dos itens 1–5 e release/site (dono 2026-10-24).

## Ordem sugerida

1. Agora: DP-1 (#542), CP-1 (#537), CI-1 nos outros repos.
2. Em paralelo: CP-2/3/4 (Jest, Playwright, PHPUnit com falha), AC-1, RT-1, UX-2, RT-3 (spike).
3. Depois: AC-2, AC-3 → UX-1; AC-4 quando o dono decidir.
4. Prova ao vivo (RT-4, DP-2) com o dono.
5. Corte (DP-3), RL-1..3, release (RL-4), site (RL-5), e só depois DP-4.
