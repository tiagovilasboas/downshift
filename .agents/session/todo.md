# Todo — Downshift MVP (atualizado em 2026-10-29, após o merge do MVP)

Plano de execução: [mvp-plan](../../docs/internal/plans/mvp-plan.md). Estado detalhado e prova de cada item: [cards do MVP, seção "Closing state (2026-10-29)"](../../docs/internal/tasks/2026-10-27-mvp/README.md).

> 📦 *Histórico concluído (ondas 0 a 1.5) arquivado em [completed-tasks-2026-10-28.md](archive/completed-tasks-2026-10-28.md)*

## MVP: etapas 0 a 3

> 📦 *Histórico compactado: 8 tarefas concluídas arquivadas em [completed-tasks-2026-10-29.md](archive/completed-tasks-2026-10-29.md)*
- [ ] **Etapa 3, provar:** V1 construído (nota de roteamento no `report`); V2 aberto (a diferença de tokens troca de sinal entre janelas); V3 parcial (padrão ligado, nota de migração em rascunho).

## Feito em 2026-10-29

- [x] Rota de escrita (bypass), desligada por padrão, em worktree git própria. Testada com shims, não contra a CLI real.
- [x] Registro de decisão e estado dos docs de planejamento (este arquivo, state, decisions, cards, mvp-plan, roadmap, CHANGELOG).

## Pendências que dependem do dono

- [x] **Mesclar a branch no `main`** feito: PR #589 mesclado em `1369aa3b` (squash), depois de o CI do head passar.
- [ ] **V2, baseline de aceitação:** suíte controlada, com gasto de modelo. Precisa de orçamento aprovado.
- [ ] **C0 / R0b:** prova em sessão real de que o PreToolUse dispara em spawns de workflow. Não rodada.
- [ ] **R27:** observar um worker de leitura tentando escrever e sendo bloqueado. Três execuções reais não tentaram escrever; nada foi observado.
- [ ] **Aceite** de K1 a K7 e de V1.
- [ ] **True north (`docs/true-north.md`):** o experimento de quatro braços (seção 6) é o V2 ampliado; precisa do mesmo orçamento. Antes de citar fora do repositório, conferir a tabela do system card do Opus 4.5 no PDF (não extraído aqui) e reler as duas páginas da OpenAI (HTTP 403).

## Pendências técnicas

- [x] **Inspector v2 e ciclo do worker** validados com `mcpdo` (Node 22) e um `claude` falso, sem gasto de tokens: leitura, escrita em worktree, `ack` e cancelamento. Ver `docs/evidence/mcp-worker-lifecycle-2026-10-29.md`. Falta o mesmo com o `claude` e o `codex` reais (R27 segue sem observação).
- [x] **Cancelar deixava o filho do worker rodando.** O worker agora roda em grupo de processos próprio e o cancelamento atinge o grupo todo, com teste de regressão.
- [ ] **A rota de escrita deixa `.downshift/` como arquivo não rastreado** no checkout do usuário. Excluir a pasta das worktrees (por exemplo em `.git/info/exclude`) ou criá-la fora do repositório.
- [ ] **A fila não tem encerramento:** ao sair o servidor, workers em execução não são parados (visto no código, não observado). Adicionar um `Close` chamado ao encerrar o `mcp serve`.
- [x] **Título do `delegate`** agora diz "Start delegated workers" (antes dizia "read-only workers" mesmo com a rota de escrita ligada). Resta decidir se `cancel` merece `destructiveHint`.
- [ ] **Ciclo do worker em worktree:** a rota de escrita cria a worktree em `internal/workers/write.go`, não em `internal/worktree` (que tem só contenção, git e slug). Decidir se a rota passa a usar o pacote e rodar o ciclo ponta a ponta com sensor de aceite.
- [ ] **Golden files do compactor:** o layout real é `<caso>.in` e `<caso>.golden`, com a flag `-update-failed-golden`, só para falhas do `go test` (6 casos). Expandir para as outras famílias da allowlist seguindo esse padrão.

- [ ] **`downshift mcp serve` perde respostas se o stdin fecha logo depois das requisições.** Com o stdin aberto, as 7 ferramentas são listadas. Clientes reais mantêm o stdin aberto; scripts e testes que fecham a entrada cedo não recebem nada. Causa no EOF do SDK oficial, não corrigida (exige mexer no transporte).

- [x] `downshift session --pause --session=<id>` funciona (verificado em 2026-10-29).
- [ ] Fazer o hint `[tier=...]` subir também (hoje só desce).
- [ ] Remover `policy economy` e sua CLI (`cmd/downshift/policy_economy.go`). O8: age só na política declarada; remoção pendente.
- [ ] Avaliar `internal/cascade`: `loop` e `exec` ainda importam. Remover só se o loop não precisar.
- [ ] **Dinheiro na saída (O1):** o `stats` já mostra só tokens e percentuais; a ajuda deixou de dizer "list price". Resta `doctor` `checkPrices` (a tabela de preços serve só de sinal de roteamento, B8).
- [ ] Guias `install.md` e `quickstart.md` já sem `downshift try`. Outros docs em `docs/` ainda citam `try`; varrer.
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
