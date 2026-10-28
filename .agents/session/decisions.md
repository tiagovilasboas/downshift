# Architectural Decisions

- **Branch Exclusiva:** `downshift/mvp-ponto-zero`. Sem PRs paralelos ou branches temporárias.
- **Autoria:** Estritamente `Tiago Vilas Boas`. Sem menções a IA ou `Co-Authored-By`.
- **Dogfooding:** Todo comando allowlisted executado via `downshift compactor -- <cmd>`.
- **SRP & Arquitetura:** Máximo de ~300 linhas por arquivo (teto 500), ~10 funções por arquivo. Separação por domínio de ação.
- **Fail-Open & Local-First:** Nenhuma falha em hooks ou discovery deve travar a execução do desenvolvedor ou expor dados sensíveis.
- **O8 (2026-10-28, dono): o LLM escolhe, o Downshift aplica e mede.** Sem tabela de perfis padrão no binário (N11 revogada), sem escalonamento automático após falha e sem piso de risco embutido. Defaults de modelo vão para um playbook do orquestrador (skill instalada pelo setup). O Downshift aplica `policy.json` declarado e o hint do orquestrador. O MCP de delegação volta a ser escopo (O7 reaberta), parte do mesmo binário (`downshift mcp`).
- **Subagent Force no Claude Code (a rever, O8):** o setup grava `CLAUDE_CODE_SUBAGENT_MODEL_FORCE=1`, que sobrepõe o modelo escolhido pelo LLM. Pendente de remoção (R1b).
- **Ordem de Entrega:** Fechar a Onda 1.5 / Release Readiness (W7.1, W7.2, S1, S2, W7.9) antes da automação em lote da Onda 2 (Batch Loop Runner).
