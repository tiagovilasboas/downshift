# Architectural Decisions

- **Branch Exclusiva:** `downshift/mvp-ponto-zero`. Sem PRs paralelos ou branches temporárias.
- **Autoria:** Estritamente `Tiago Vilas Boas`. Sem menções a IA ou `Co-Authored-By`.
- **Dogfooding:** Todo comando allowlisted executado via `downshift compactor -- <cmd>`.
- **SRP & Arquitetura:** Máximo de ~300 linhas por arquivo (teto 500), ~10 funções por arquivo. Separação por domínio de ação.
- **Fail-Open & Local-First:** Nenhuma falha em hooks ou discovery deve travar a execução do desenvolvedor ou expor dados sensíveis.
- **Alinhamento com Business Plan (N11):** Roteamento padrão baseado na Tabela de Perfis Padrão embutida no binário (`Explore` -> small, `claude-code-guide` -> small, `statusline-setup` -> small; `general-purpose` e `Plan` mantêm solicitado).
- **Subagent Force no Claude Code:** Setup configura nativamente `"CLAUDE_CODE_SUBAGENT_MODEL_FORCE": "1"` no `settings.json`.
- **Ordem de Entrega:** Fechar a Onda 1.5 / Release Readiness (W7.1, W7.2, S1, S2, W7.9) antes da automação em lote da Onda 2 (Batch Loop Runner).
