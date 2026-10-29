
### 📦 Tarefas Compactadas em 2026-10-29 01:22:50 (8 itens)
- [x] **Etapa 0, base limpa:** B0 a B9 (v1 removido, contrato de roteamento, política declarada, dinheiro fora).
- [x] **Etapa 1, setup:** S1 marcadores de posse, S2 skill instalada, S3 `doctor` prova que o hook disparou.
- [x] **Etapa 2, modelo certo no Claude Code:** C1 a C4 no branch. Falta C0 (prova em sessão real).
- [x] **Etapa 2b, ferramentas do orquestrador (MCP):** K1 a K7 feitos; aguardam aceite do dono. Ferramentas `providers`, `playbook`, `status`, `apply`; registro com `setup --mcp`.
- [x] Hook `SessionStart` com contexto para o orquestrador: chamar `providers` antes de spawn e usar `[tier=small]` no trabalho mecânico (`066589db`). Também em `~/.claude/settings.json`, com cópia de segurança `settings.json.bak-sessionstart-2026-10-29`.
- [x] Descrição do plugin sem "roteador determinístico" (`7e2acb02`).
- [x] Ferramentas de trabalho do MCP: `delegate`, `collect`, `cancel`. Fila com limite de 4, orçamento de tempo por tarefa, entregas com `ack`. Testada com workers falsos.
- [x] Rota de leitura: um worker Claude completou delegate→collect de ponta a ponta. A prova não ficou em `docs/evidence/`.
