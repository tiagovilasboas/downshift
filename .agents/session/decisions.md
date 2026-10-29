# Architectural Decisions

- **Branch Exclusiva:** `downshift/mvp-ponto-zero`. Sem PRs paralelos ou branches temporárias.
- **Autoria:** Estritamente `Tiago Vilas Boas`. Sem menções a IA ou `Co-Authored-By`.
- **Dogfooding:** Todo comando allowlisted executado via `downshift compactor -- <cmd>`.
- **SRP & Arquitetura:** Máximo de ~300 linhas por arquivo (teto 500), ~10 funções por arquivo. Separação por domínio de ação.
- **Fail-Open & Local-First:** Nenhuma falha em hooks ou discovery deve travar a execução do desenvolvedor ou expor dados sensíveis.
- **O8 (2026-10-28, dono): o LLM escolhe, o Downshift aplica e mede.** Sem tabela de perfis padrão no binário (N11 revogada), sem escalonamento automático após falha e sem piso de risco embutido. Defaults de modelo vão para um playbook do orquestrador (skill instalada pelo setup). O Downshift aplica `policy.json` declarado e o hint do orquestrador. O MCP de delegação volta a ser escopo (O7 reaberta), parte do mesmo binário (`downshift mcp`).
- **Subagent Force no Claude Code (resolvido, O8):** o setup deixou de gravar `CLAUDE_CODE_SUBAGENT_MODEL_FORCE=1` (`0c4574a2`). Um valor definido pelo próprio usuário é preservado.
- **Rota de workers (2026-10-29, dono):** leitura ligada por `DOWNSHIFT_WORKERS=read`; escrita (bypass) só com `DOWNSHIFT_WORKERS=bypass`, sempre em worktree git própria. Padrão desligado.
- **Ordem de Entrega (2026-10-29):** MVP (etapas 0 a 3, com a 2b) antes do plano runner (etapa 4) e dos outros harnesses (etapa 5). O Batch Loop Runner da Onda 2 fica adiado. Ver [mvp-plan](../../docs/internal/plans/mvp-plan.md).
- **Estratégia de Testes Sem Baterias Sintéticas (2026-10-29, dono):** Sem simulações complexas de diálogos de LLM ou mocks frágeis de harnesses. Conformidade MCP via `@modelcontextprotocol/inspector` oficial; compactor via golden files (`*.raw` -> `*.golden`); integridade arquitetural via `internal/archtest` (<2s); validação de subagente por gate externo objetivo (`exit code == 0` no teste alvo + `git diff` no escopo). Ver `docs/internal/plans/universal-orchestrator-harness-approach.md`.
