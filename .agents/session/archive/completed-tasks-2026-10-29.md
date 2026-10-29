
### 📦 Tarefas Compactadas em 2026-10-29 01:22:50 (8 itens)
- [x] **Etapa 0, base limpa:** B0 a B9 (v1 removido, contrato de roteamento, política declarada, dinheiro fora).
- [x] **Etapa 1, setup:** S1 marcadores de posse, S2 skill instalada, S3 `doctor` prova que o hook disparou.
- [x] **Etapa 2, modelo certo no Claude Code:** C1 a C4 no branch. Falta C0 (prova em sessão real).
- [x] **Etapa 2b, ferramentas do orquestrador (MCP):** K1 a K7 feitos; aguardam aceite do dono. Ferramentas `providers`, `playbook`, `status`, `apply`; registro com `setup --mcp`.
- [x] Hook `SessionStart` com contexto para o orquestrador: chamar `providers` antes de spawn e usar `[tier=small]` no trabalho mecânico (`066589db`). Também em `~/.claude/settings.json`, com cópia de segurança `settings.json.bak-sessionstart-2026-10-29`.
- [x] Descrição do plugin sem "roteador determinístico" (`7e2acb02`).
- [x] Ferramentas de trabalho do MCP: `delegate`, `collect`, `cancel`. Fila com limite de 4, orçamento de tempo por tarefa, entregas com `ack`. Testada com workers falsos.
- [x] Rota de leitura: um worker Claude completou delegate→collect de ponta a ponta. A prova não ficou em `docs/evidence/`.

### 📦 Tarefas Compactadas em 2026-10-29 03:49:27 (7 itens)
- [x] Rota de escrita (bypass), desligada por padrão, em worktree git própria. Testada com shims, não contra a CLI real.
- [x] Registro de decisão e estado dos docs de planejamento (este arquivo, state, decisions, cards, mvp-plan, roadmap, CHANGELOG).
- [x] **Mesclar a branch no `main`** feito: PR #589 mesclado em `1369aa3b` (squash), depois de o CI do head passar.
- [x] **Inspector v2 e ciclo do worker** validados com `mcpdo` (Node 22) e um `claude` falso, sem gasto de tokens: leitura, escrita em worktree, `ack` e cancelamento. Ver `docs/evidence/mcp-worker-lifecycle-2026-10-29.md`. Falta o mesmo com o `claude` e o `codex` reais (R27 segue sem observação).
- [x] **Cancelar deixava o filho do worker rodando.** O worker agora roda em grupo de processos próprio e o cancelamento atinge o grupo todo, com teste de regressão.
- [x] **A rota de escrita deixava `.downshift/` como arquivo não rastreado.** A pasta das worktrees agora entra no `.git/info/exclude` do clone, e o teste da rota olha o `git status`.
- [x] **A fila não tinha encerramento.** O `mcp serve` chama `Queue.Close` ao sair e nenhum worker nem filho sobra (conferido por um cliente MCP real).
