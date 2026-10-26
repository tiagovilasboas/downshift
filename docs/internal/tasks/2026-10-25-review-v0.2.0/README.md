<!-- Copyright (c) 2026 Tiago Vilas Boas. SPDX-License-Identifier: Apache-2.0 -->

# Code review final pós v0.2.0 (2026-10-25): tarefas

Downshift by Tiago Vilas Boas · https://github.com/tiagovilasboas/downshift

Origem: review de tudo o que entrou desde a v0.1.3 (KiroCrew, `limits`, contabilidade,
release v0.1.4 e v0.2.0) e do repositório labs. Nota geral: **6,5/10**. Medido em
2026-10-25: `make check` verde no `origin/main` (`55c7db94`); suíte `harnesssim` verde
contra binário novo; `downshift report` desta máquina com 1441 comandos, 106 compactados.

Legenda igual à do [roadmap](../2026-10-24-roadmap/README.md): **ABERTO**, **DECISÃO**
(depende do dono). Pontos Fibonacci, teto 5 por tarefa. Cada tarefa é um PR próprio.

## Ordem

Fase 1 (bugs que afetam dado): RV2-1, RV2-2, RV2-3. Fase 2 (higiene): RV2-4 a RV2-8.
Fase 3 (estrutura e medição): RV2-9 a RV2-11. RV2-1 e RV2-2 não dependem de nada e podem
rodar em paralelo. RV2-3 espera a decisão de editar a release publicada.

## Tarefas

| ID | Gravidade | Tarefa | Estado | Arquivos | Pronto quando | Depende | Pts |
|---|---|---|---|---|---|---|--:|
| RV2-1 | P1 | Leitor de log do KiroCrew para no primeiro linha acima de 64 KiB, e o modelo sai errado. Nos 3 logs reais: `opus-4.5` no lugar de `sonnet-4.6`, e `unknown` em 2 casos | ABERTO | `internal/discovery/kirocrew.go` (`modelFromSessionLog`, `maxJSONLBytes`), `kirocrew_test.go` | Teste com linha de 100 KB e fixture no formato real devolve o último modelo; leitura sem teto de linha, ou lendo do fim; teste falha antes do fix | — | 2 |
| RV2-2 | P1 | Registro `internal/limits` contradiz a matriz (codex e claude-code como "full", antigravity e opencode como "full" sem spawn observado). Doctor só lista o que não é "full". Fix aponta `downshift help <harness>` (só mostra o hook) e o texto cita `compactor --` em vez de `compact --` | ABERTO | `internal/limits/limits.go`, `limits_test.go`, `cmd/downshift/doctor_hooks.go` (`checkHarnessLimitations`) | Fatos vêm de `HarnessCapabilities` ou da matriz, com teste de paridade com `docs/reference/harness-matrix.md`; doctor mostra `Experimental`/`Beta` como aviso; comando correto no fix | — | 3 |
| RV2-3 | P1 | Release: CHANGELOG sem seções `[0.1.4]` e `[0.2.0]`; README diz "Beta (v0.1.3)"; CHANGELOG cita `sessions/<id>/state.json`, o código lê `subagents/<id>/state.json` e `sessions/<parent>.jsonl`; notas da v0.2.0 anunciam `EstimateContextSavings` como feature; roadmap v0.2.0 marca "Released" sem tag de checagem. v0.1.4 e v0.2.0 saíram com 40 min de diferença | ABERTO + DECISÃO (editar a release publicada v0.2.0 com `gh release edit`) | `CHANGELOG.md`, `README.md`, `docs/roadmap-v0.2.0.md` | Seções de versão criadas; README aponta a versão real; caminhos corrigidos; notas da v0.2.0 batem com o código | DECISÃO do dono sobre a edição da release | 2 |
| RV2-4 | P2 | Código morto do KiroCrew: `InferKiroCrewMetrics`, `KiroSessionMetrics`, `accounting.KiroCrewSessionMetrics`, `NewKiroCrewSessionMetrics`, `EstimateContextSavings` não têm chamador fora de teste. `execwrap.Event.Model` é gravado e nunca lido. `EstimateContextSavings` usa `/4` fixo e não o `BytesPerToken` | ABERTO | `internal/discovery/kirocrew.go`, `internal/accounting/kirocrew.go`, `types.go`, `internal/execwrap/metrics.go` | Ou ligado ao `Compute` (fonte única) com teste de paridade, ou removido; `git grep` sem órfão | RV2-1 (mesmo arquivo) | 2 |
| RV2-5 | P2 | `envModel` roda a cada exec e varre todos `subagents/*` e o log inteiro (7 a 18 MB por log). O cache de processo não ajuda num hook curto | ABERTO | `cmd/downshift/session.go` (`envModel`), `cmd/downshift/exec.go` | Medir: tempo por exec com log de 18 MB antes e depois, registrado no PR; sem varredura completa no caminho quente | RV2-1 | 2 |
| RV2-6 | P2 | Caminhos pessoais `/Users/tiago...` em 13 arquivos rastreados, incluindo `scripts/curate-rea.sh`, script de migração único e não portável | ABERTO | 13 arquivos (`git grep -l '/Users/tiago'`), `scripts/curate-rea.sh` | `git grep '/Users/'` vazio em arquivos rastreados (testes usam `t.TempDir`); `curate-rea.sh` removido ou com caminhos por variável | — | 2 |
| RV2-7 | P2 | 110 referências a `downshift-labs`, repositório privado, em docs públicos, incluindo links `blob/` de evidência na matriz (404 para quem não é dono) | ABERTO + DECISÃO (copiar evidência para o público ou marcar como privada) | `docs/README.md`, `docs/reference/harness-matrix.md`, `docs/guides/*`, demais | Nenhum link público quebrado; cada referência privada virou texto ou evidência pública | DECISÃO do dono | 3 |
| RV2-8 | P2 | Nome completo ("Tiago de Carvalho Vilas Boas") em ao menos 9 arquivos do labs, incluindo `eval/FREEZE.json` e cabeçalho de `scripts/baseline_eval.py`. Viola a regra primordial de identidade, mesmo no repo privado | ABERTO + DECISÃO (`FREEZE.json` é artefato pré-registrado: trocar o texto precisa de registro) | labs: 9 arquivos (`git grep -i carvalho`) | `git grep -i carvalho` vazio; `FREEZE.json` com nota de alteração registrada, aprovada pelo dono | DECISÃO do dono (FREEZE) | 2 |
| RV2-9 | P3 | Arquivos acima de 500 linhas (regra rígida): `codeintel/parse_lang.go` 1244, `compressor/compressor.go` 1099, `telemetry/telemetry.go` 916 (ver AC-2), `catalog/catalog.go` 869, `execwrap/policy_tooling.go` 851 com 19 famílias, `codeintel/security.go` 682 | ABERTO | os 6 arquivos acima | Nenhum arquivo não-teste, não gerado, acima de 500; um PR por arquivo, divisão por domínio de ação | RV2-4 onde houver sobreposição | 5 (cada) |
| RV2-10 | P3 | Repo labs: o CI roda só `ci-eval.yml`; a suíte `harnesssim` usa `../downshift/downshift` e pode medir um binário velho | ABERTO | `labs/.github/workflows/`, `labs/Makefile` | Job que compila o binário do `main` do downshift e roda `make harnesssim` | — | 2 |
| RV2-11 | P3 | Número de capa do `downshift report` ("tokens saved") é 99,6% `cache_saved` (23,8 M de 23,9 M). Work tokens são 55,7 K e compactor 47 K. O rótulo promete economia que é, em boa parte, tokens em modelo mais barato | ABERTO + DECISÃO (rótulo ou mostrar componentes separados) | `internal/accounting` (formula), `cmd/downshift/report*.go`, `session --panel` | Report mostra work, compactor e cache em linhas separadas; rótulo de capa só com componentes que são de fato economia de tokens; teste de paridade com `Compute` | DECISÃO do dono sobre o rótulo | 3 |

## Decisões que só o dono toma

1. Editar a release publicada v0.2.0 (notas e CHANGELOG) ou abrir v0.2.1 (RV2-3).
2. Copiar a evidência do labs para o público ou manter os links marcados como privados (RV2-7).
3. Trocar o nome completo em `eval/FREEZE.json` e demais arquivos do labs (RV2-8).
4. Rótulo do número de capa do report (RV2-11).

## Medição de fechamento

Ao fechar cada PR, registrar estimado contra real (minutos e chamadas de ferramenta) e a
diferença para a próxima. A lição deste review: três PRs e uma release em menos de uma hora,
validados só com testes sintéticos. RV2-1 teria sido pego por uma sonda com log real antes
do merge. Para qualquer mudança no KiroCrew, rodar a sonda contra `~/.kiro/crew` antes de
abrir o PR.
