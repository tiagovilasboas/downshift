# Session State

- **Branch atual:** `docs/universal-harness-roadmap` baseada em `origin/main` (após MVP mesclado no PR #589).
- **Fase atual:** Documentação de governança, estratégia de testes e abordagem do Orquestrador Universal / MCP / Harness Engineering.
- **Status:** Criado `docs/internal/plans/universal-orchestrator-harness-approach.md`, atualizados `docs/internal/plans/roadmap.md` e `mvp-plan.md`. `docscheck` verde.
- **Próximo passo:** Finalizar validação de `make check`, preparar commit em inglês e orientar o uso do Haiku 5.5 mecânico com os prompts operacionais.
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
