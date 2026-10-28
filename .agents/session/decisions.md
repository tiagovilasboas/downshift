# Architectural Decisions

- **Branch Exclusiva:** `downshift/mvp-ponto-zero`. Sem PRs paralelos ou branches temporárias.
- **Autoria:** Estritamente `Tiago Vilas Boas`. Sem menções a IA ou `Co-Authored-By`.
- **Dogfooding:** Todo comando allowlisted executado via `downshift compactor -- <cmd>`.
- **SRP & Arquitetura:** Máximo de ~300 linhas por arquivo (teto 500), ~10 funções por arquivo. Separação por domínio de ação.
- **Fail-Open & Local-First:** Nenhuma falha em hooks ou discovery deve travar a execução do desenvolvedor ou expor dados sensíveis.
