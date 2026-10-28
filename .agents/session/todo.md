# Todo List — Downshift Roadmap & Ondas

## Onda 0: Fundação & Setup Unificado (Concluída)
> 📦 *Histórico compactado: 4 tarefas arquivadas em [completed-tasks-2026-10-28.md](archive/completed-tasks-2026-10-28.md)*

## Onda 1: MVP Funcional no Claude Code & Loop Engineer (Concluída)
> 📦 *Histórico compactado: 3 tarefas arquivadas em [completed-tasks-2026-10-28.md](archive/completed-tasks-2026-10-28.md)*
- [x] F1.4: Validação E2E no Claude Code (bateria dos 4 hooks)
- [x] F1.5: Inventário de Cobertura de Pacotes (`docs/contrib/coverage.md`)
- [x] F1.6: Loop Engineer Engine CLI (`downshift loop`) com ciclo Plan -> Implement -> Sensor -> Cascade -> Accept
- [x] Correção de concorrência multi-processo com Directory Lock atômico em `dead_ends`
- [x] Injeção de `CLAUDE_CODE_SUBAGENT_MODEL_FORCE` via `downshift setup`

## Onda 1.5: Setup Completo & Release Readiness (Business Plan W7 & Stage 1)
- [x] **W7.1 (3 pts):** Default Profile Table (N11) embutida no binário — mapeamento de tipos padrão (`Explore` -> small, `claude-code-guide` -> small, `statusline-setup` -> small; `general-purpose` e `Plan` mantidos), sem risco
- [ ] **W7.2 (2 pts):** `downshift policy init` integrado no `setup` — gera `.downshift/policy.json` com confirmação e preview
- [ ] **S1 (3 pts):** Ownership Markers (`.downshift-managed`) nos arquivos gerados pelo setup para suporte robusto a rollback e integridade
- [ ] **S2 (3 pts):** Instalação automática da Skill do Downshift em `~/.claude/skills` e `~/.codex/skills` via setup
- [ ] **W7.9 (3 pts):** Modo Economia (`economy mode` no `policy.json` ou flag `--economy`), rebaixando um tier com segurança

## Onda 2: Cloud Software Factory & Escala Multi-Harness (W2 & Factory)
- [ ] **F2.1 (3 pts):** Batch Loop Runner (`downshift loop --batch=tasks.json`) para orquestração sequencial/isolada de tarefas
- [ ] **F2.2 (2 pts):** Relatório Consolidado de Fábrica (`downshift loop report`) com telemetria e taxa de aceitação git
- [ ] **F2.3 (3 pts):** Multi-Harness Universal Seeder com auto-descoberta de OpenCode, Cursor e Codex em `session-models.json`
- [ ] **F2.4 (5 pts):** Worktree isolado e hermético (`internal/worktree`, W2.1 do business plan)
