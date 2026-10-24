<!-- Copyright (c) 2026 Tiago Vilas Boas. SPDX-License-Identifier: Apache-2.0 -->

# Roadmap consolidado (2026-10-24)

Downshift by Tiago Vilas Boas · https://github.com/tiagovilasboas/downshift

Mapa único do que está feito, aberto e bloqueado. Cada frente aponta para o plano que a
detalha. Números de PR foram conferidos no GitHub em 2026-10-24.

## Estado

- **Merged no main:** 33 dos 35 PRs desta rodada (contabilidade, Compactor, roteamento,
  KiroCrew, CI, lefthook, downshift-progress até a API e a barra).
- **Abertos:** #542 (ferramentas do downshift-progress), #524 (orçamentos de tempo só no CI, destrava
  o pre-push), #537 (`gh run view --log` no Compactor).
- **Fechado nesta rodada:** #525 (o conserto do haiku já estava no main).

## Frentes

| Frente | Feito | Falta | Plano |
|---|---|---|---|
| **downshift-progress** | estrutura, cliente de dados, formato/i18n/cores, `progress`/`step`, roster, painel Agents, barra, campos novos do painel | #542 merge; prova lado a lado com o savvy-progress (E1, ao vivo, com o dono); corte: `setup` troca o savvy-progress e o mod entra no marketplace (M6+M7); apagar o savvy-progress após uma release (M8) | [downshift-progress](../2026-10-24-downshift-progress/README.md) |
| **Contabilidade** | conta única `Compute` (report, painel, API, mod); teste de paridade; regras de camada; fórmula corrigida | A6: apagar agregações mortas e dividir `telemetry.go` (~900 linhas) e `session_tree.go`; `stats` vira `report --days`; teto de bytes por harness; USD só depois | [accounting-v1](../2026-10-21-accounting-v1/README.md) |
| **Compactor** | `go test` limpo e com falha; `2>&1`; `VAR=` prefixo; Jest no stderr; pnpm/yarn/php; docker; orientação em todos os harnesses; reescrita de shell no Codex e no Cursor | #537 (`gh run view --log`); falhas do Jest, Playwright e PHPUnit; `npm run <script>`; cobertura do Jest; `pnpm build`; pular saídas pequenas de git | [minimal-wins](../2026-10-23-minimal-wins/README.md) |
| **Roteamento** | calibração do classificador; tipo do agente como sinal; KiroCrew (ferramenta de spawn, modelo servido, lote do `spawn_run`) | K4: mensagem de respawn na camada compartilhada; descoberta automática de modelos por sessão (spike ainda não feito); decisão C6 (upshift acima de modelo pedido explicitamente) | [claude-code-and-ux](../2026-10-23-claude-code-and-ux/README.md) |
| **UX dos comandos** | `--help` em todo comando; erro de comando desconhecido em uma linha; `compact` no `help`; `session` em palavras simples; `doctor` curto | `report` começando pela resposta; `--json` em `models` e `feedback` | [claude-code-and-ux](../2026-10-23-claude-code-and-ux/README.md) |
| **CI e estabilidade** | Windows; `make check`; lefthook (pre-commit com lint e testes); `merge_group`; guarda no harness-core (`ci-health`, `merge-when-green`, `local-ci`, regra `local-gate-first`) | #524 merge (destrava pre-push e #542); aplicar hooks nos outros repos | [spike CI](../2026-10-23-spike-progress-kiro-accounting/README.md) |
| **Release v0.1.4 e site** | nada ainda | CHANGELOG (`[Unreleased]` com tudo acima); README (ainda diz beta v0.1.1); matriz de harness; tag; PR no `downshift-site` | — |

## Bloqueios que dependem do dono

- **Cobrança do GitHub Actions:** repos privados (harness-core, opendraw, voomp-e2e, montanha-tech-site) não rodam CI.
- **Provas ao vivo:** Codex, Cursor e o downshift-progress lado a lado com o savvy-progress.
- **voomp-e2e #13:** 14 specs batem em sites de produção; não rodar de novo sem revisão.
- **C6:** o roteamento pode subir acima de um modelo que o pai pediu de forma explícita?

## Ordem

1. #524 e #542 entram.
2. Prova ao vivo (Codex, Cursor, downshift-progress).
3. Corte do downshift-progress (M7).
4. A6 e o `report` com a resposta primeiro.
5. Release v0.1.4, site e descoberta de modelos por sessão.
