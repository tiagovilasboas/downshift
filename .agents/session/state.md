# Session State

- **Branch atual:** `downshift/mvp-ponto-zero` (publicada em `origin`; o `main` não foi tocado).
- **Fase atual:** MVP, etapa 3 (V1 construído, V2 aberto, V3 parcial), com a etapa 4 parcial pelo caminho MCP.
- **Status:** docs de planejamento (todo, cards do MVP, plano do MVP, roadmap, CHANGELOG) alinhados com o estado de 2026-10-29.
- **Próximo passo:** decisão do dono sobre mesclar a branch no `main` (chega ao plugin o hook `SessionStart`), orçamento da suíte V2 e a prova C0 em sessão real.
- **Bloqueios:** nenhum técnico. Os itens acima aguardam aprovação ou orçamento do dono.
- **Commits recentes na branch:**
  - `7e2acb02`: descrição do plugin como escolha do orquestrador
  - `066589db`: contexto de `SessionStart` pedindo `providers` ao orquestrador
  - `c5d51973`: estado de fechamento do MVP medido, não provado
  - `2fa8ca7c`: rota de escrita com bypass, opt-in, em worktree própria
  - `8653a085`: leituras somente leitura para Claude e Codex atrás de uma flag

## Estado em 2026-10-28 (O8)

- Decisão do dono: o LLM orquestrador escolhe agente, provedor, modelo e esforço. O Downshift aplica dentro da política declarada e mede. Removidos: tabela N11, escalonamento automático após falha, veto de risco embutido e a escrita de `CLAUDE_CODE_SUBAGENT_MODEL_FORCE` no setup.
- Novos: playbooks de regras de trabalho (`internal/playbook`, `downshift playbook`), skill `downshift-orchestrator` e o servidor MCP (`downshift mcp serve`). O `setup --mcp` registra no Claude Code e no Codex.
