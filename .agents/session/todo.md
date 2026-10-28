# Todo List — Downshift Roadmap & Ondas

## Onda 0: Fundação & Setup Unificado (Concluída)
> 📦 *Histórico compactado: 4 tarefas arquivadas em [completed-tasks-2026-10-28.md](archive/completed-tasks-2026-10-28.md)*

## Onda 1: MVP Funcional no Claude Code & Loop Engineer (Concluída)
> 📦 *Histórico compactado: 3 tarefas arquivadas em [completed-tasks-2026-10-28.md](archive/completed-tasks-2026-10-28.md)*
> 📦 *Histórico compactado: 5 tarefas concluídas arquivadas em [completed-tasks-2026-10-28.md](archive/completed-tasks-2026-10-28.md)*

## Onda 1.5: Setup Completo & Release Readiness (Business Plan W7 & Stage 1)
- [x] **W7.1 (3 pts):** Default Profile Table (N11) embutida no binário — mapeamento de tipos padrão (`Explore` -> small, `claude-code-guide` -> small, `statusline-setup` -> small; `general-purpose` e `Plan` mantidos), sem risco
- [x] **W7.2 (2 pts):** `downshift policy init` integrado no `setup` — gera `.downshift/policy.json` com confirmação e preview
- [x] **S1 (3 pts):** Ownership Markers: arquivos inteiros gerados pelo setup (wrappers e steering dedicado) levam a frase `Written by \`downshift setup\``; `--undo` deixa intacto um arquivo sem a frase e o setup recusa sobrescrever arquivo não gerado por ele
- [x] **S2 (3 pts):** Instalação da skill `downshift-audit` (somente leitura) em `~/.claude/skills` e `~/.codex/skills` via setup, com marcador, backup, dry run e conflito em pasta que não é nossa
- [x] **W7.9 (3 pts):** Modo Economia (`economy mode` no `policy.json` ou flag `--economy`), rebaixando um tier com segurança

## Onda 2: Cloud Software Factory & Escala Multi-Harness (W2 & Factory)
- [x] **F2.1 (3 pts):** Batch Loop Runner (`downshift loop --batch=tasks.json`) para orquestração sequencial/isolada de tarefas
- [x] **F2.2 (2 pts):** Relatório Consolidado de Fábrica (`downshift loop report`) com telemetria e taxa de aceitação git
- [ ] **F2.3 (3 pts):** Multi-Harness Universal Seeder com auto-descoberta de OpenCode, Cursor e Codex em `session-models.json`
  - parte 1 feita: fonte `opencode models` em `internal/discovery`
  - parte 2 feita: o seeder do `setup` usa a lista ranqueada do cache de descoberta e só cai nas listas fixas sem cache; OpenCode saiu das listas fixas
  - pendente: arquivos `session-models.json` já gravados com IDs antigos de OpenCode não são reescritos (o seeder só cria entradas ausentes)
- [ ] **F2.4 (5 pts):** Worktree isolado e hermético (`internal/worktree`, W2.1 do business plan)
