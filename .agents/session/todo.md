# Todo — Downshift MVP (atualizado em 2026-10-29)

Plano de execução: [mvp-plan](../../docs/internal/plans/mvp-plan.md). Estado detalhado e prova de cada item: [cards do MVP, seção "Closing state (2026-10-29)"](../../docs/internal/tasks/2026-10-27-mvp/README.md).

> 📦 *Histórico concluído (ondas 0 a 1.5) arquivado em [completed-tasks-2026-10-28.md](archive/completed-tasks-2026-10-28.md)*

## MVP: etapas 0 a 3

> 📦 *Histórico compactado: 8 tarefas concluídas arquivadas em [completed-tasks-2026-10-29.md](archive/completed-tasks-2026-10-29.md)*
- [ ] **Etapa 3, provar:** V1 construído (nota de roteamento no `report`); V2 aberto (a diferença de tokens troca de sinal entre janelas); V3 parcial (padrão ligado, nota de migração em rascunho).

## Feito em 2026-10-29

- [x] Rota de escrita (bypass), desligada por padrão, em worktree git própria. Testada com shims, não contra a CLI real.
- [x] Registro de decisão e estado dos docs de planejamento (este arquivo, state, decisions, cards, mvp-plan, roadmap, CHANGELOG).

## Pendências que dependem do dono

- [ ] **Mesclar a branch no `main`** para o hook `SessionStart` chegar ao plugin. Hoje o plugin está fixado em `3f37b9d5`. Exige aprovação explícita; o `main` já andou à frente.
- [ ] **V2, baseline de aceitação:** suíte controlada, com gasto de modelo. Precisa de orçamento aprovado.
- [ ] **C0 / R0b:** prova em sessão real de que o PreToolUse dispara em spawns de workflow. Não rodada.
- [ ] **R27:** observar um worker de leitura tentando escrever e sendo bloqueado. Três execuções reais não tentaram escrever; nada foi observado.
- [ ] **Aceite** de K1 a K7 e de V1.
- [ ] **True north (`docs/true-north.md`):** o experimento de quatro braços (seção 6) é o V2 ampliado; precisa do mesmo orçamento. Antes de citar fora do repositório, conferir a tabela do system card do Opus 4.5 no PDF (não extraído aqui) e reler as duas páginas da OpenAI (HTTP 403).

## Pendências técnicas

- [ ] Verificar `downshift session --pause --session=<id>`: o id não foi respeitado (pausa a sessão mais recente). Não reverificado em 2026-10-29.
- [ ] Fazer o hint `[tier=...]` subir também (hoje só desce).
- [ ] Remover `policy economy` e sua CLI (`cmd/downshift/policy_economy.go`). O8: age só na política declarada; remoção pendente.
- [ ] Avaliar `internal/cascade`: `loop` e `exec` ainda importam. Remover só se o loop não precisar.
- [ ] **Dinheiro na saída (conflito com O1):** `downshift stats` diz "estimated savings (list price)" e `doctor` tem `checkPrices`. O1 proíbe contabilidade em dólar. A tabela de preços fica só como sinal de roteamento (B8).
- [ ] **Guia de instalação desatualizado:** `docs/guides/install.md` e `docs/guides/quickstart.md` usam `downshift try`, comando já removido. Também descrevem o classificador e `claude-haiku-4-5`. Reescrever como PR próprio.
- [ ] Varrer `75%` e `haiku-4-5` nos docs de produto (lista em `docs/`). Registros históricos de evidência ficam como estão.
- [ ] Ativos de marca com "No LLM in the loop" (SVG) e o repositório `downshift-business-plan`.
- [ ] Suíte Aider de 39 tarefas (no scratchpad, fora do repo): o exercício `counter` passa com o stub; corrigir antes de usar em V2. O repositório do benchmark não declara licença.

## Etapa 4, plano runner (parcial)

- [x] Já construído pelo caminho MCP: fila de workers, lançamento de leitura Claude e Codex, rota de escrita em worktree (partes de P3 e P5).
- [ ] P1 schema e `plan validate`; P2 `plan dry-run`; P6 admissão por orçamento; P7 escolha de provedor; P8 escalonamento opt-in. Não iniciados.
- [ ] Lançamento de leitura do Codex: não rodado contra a CLI real.

## Etapa 5 e adiados (fora do MVP)

- [ ] Canal Codex (A3) e Kiro (A4); depois OpenCode, Cursor e Antigravity, um PR cada.
- [ ] F2.3 seeder multi-harness: partes 1 e 2 feitas; falta reescrever `session-models.json` antigos com IDs de OpenCode.
- [ ] F2.4 worktree isolado do loop (`internal/worktree`): S0 a S2 feitos; S3 a S5 esperam a política de órfãos (R34).
- [ ] Batch Loop Runner da Onda 2: adiado até o plano runner.
