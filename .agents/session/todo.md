# Todo List — Downshift Roadmap & Ondas

## Onda 0: Fundação & Setup Unificado (Concluída)
> 📦 *Histórico compactado: 4 tarefas arquivadas em [completed-tasks-2026-10-28.md](archive/completed-tasks-2026-10-28.md)*

## Onda 1: MVP Funcional no Claude Code & Loop Engineer (Concluída)
> 📦 *Histórico compactado: 3 tarefas arquivadas em [completed-tasks-2026-10-28.md](archive/completed-tasks-2026-10-28.md)*
> 📦 *Histórico compactado: 5 tarefas concluídas arquivadas em [completed-tasks-2026-10-28.md](archive/completed-tasks-2026-10-28.md)*

## Onda 1.5: Setup Completo & Release Readiness (Business Plan W7 & Stage 1)
- [x] **W7.1 (3 pts):** Default Profile Table (N11) embutida no binário — mapeamento de tipos padrão (`Explore` -> small, `claude-code-guide` -> small, `statusline-setup` -> small; `general-purpose` e `Plan` mantidos), sem risco **[Removido 2026-10-28 (O8): defaults foram para o playbook do orquestrador]**
- [x] **W7.2 (2 pts):** `downshift policy init` integrado no `setup` — gera `.downshift/policy.json` com confirmação e preview
- [x] **S1 (3 pts):** Ownership Markers: arquivos inteiros gerados pelo setup (wrappers e steering dedicado) levam a frase `Written by \`downshift setup\``; `--undo` deixa intacto um arquivo sem a frase e o setup recusa sobrescrever arquivo não gerado por ele
- [x] **S2 (3 pts):** Instalação da skill `downshift-audit` (somente leitura) em `~/.claude/skills` e `~/.codex/skills` via setup, com marcador, backup, dry run e conflito em pasta que não é nossa
- [x] **W7.9 (3 pts):** Modo Economia (`economy mode` no `policy.json` ou flag `--economy`), rebaixando um tier com segurança **[Superado por O8: age só na política declarada; remoção pendente]**

## Onda 2: Cloud Software Factory & Escala Multi-Harness (W2 & Factory)
- [x] **F2.1 (3 pts):** Batch Loop Runner (`downshift loop --batch=tasks.json`) para orquestração sequencial/isolada de tarefas
- [x] **F2.2 (2 pts):** Relatório Consolidado de Fábrica (`downshift loop report`) com telemetria e taxa de aceitação git
- [ ] **F2.3 (3 pts):** Multi-Harness Universal Seeder com auto-descoberta de OpenCode, Cursor e Codex em `session-models.json`
  - parte 1 feita: fonte `opencode models` em `internal/discovery`
  - parte 2 feita: o seeder do `setup` usa a lista ranqueada do cache de descoberta e só cai nas listas fixas sem cache; OpenCode saiu das listas fixas
  - pendente: arquivos `session-models.json` já gravados com IDs antigos de OpenCode não são reescritos (o seeder só cria entradas ausentes)
- [ ] **F2.4 (5 pts):** Worktree isolado e hermético (`internal/worktree`, W2.1 do business plan)
  - dividido em slices (pacote `internal/worktree`): S0 sensor em RepoDir (1), S1 slug e contenção (2) feito, S2 runner git hermético (2) feito, S3 lease (2), S4 provision (3), S5 verify (1); ligação ao loop com `--isolate` (2) em card separado
  - decisão aberta: política de órfãos (R34) antes de S3 e S4

## O8 (2026-10-28): o LLM escolhe, o Downshift aplica e mede
- [x] Decisor sem tabela N11, sem cascade e sem veto de risco (`b5e4eede`); setup sem FORCE (`0c4574a2`).
- [x] Playbook do orquestrador (skill) instalado pelo setup (`a70feea0`); `downshift playbook` com editor no CLI (`fa3a5c28`).
- [x] Servidor MCP `downshift mcp serve`: ferramentas de informação, modelos detectados (K1), `route` (K2), `setup --mcp` (K6) — etapa 2b do MVP feita em 2026-10-28; sessão real do Claude Code registrada em `docs/evidence/mcp-real-session-2026-10-28.md`.
- [ ] Ferramentas do MCP que lançam workers (`delegate`, `steer`, `result`, `cancel`): vêm com o runner de planos (etapa 4).
- [ ] Etapa 3 do MVP: V1 (nota de roteamento no `report`), V2 (baseline de aceitação), V3 (sessão real, padrão ligado, nota de migração); mais a prova real C0 (Opus para Haiku).
- [ ] Hint também para cima (hoje só desce); remover `policy economy` e sua CLI; limpar `internal/cascade` se o loop não precisar.
- [ ] Atualizar os ativos de marca (SVGs com "No LLM in the loop") e o repositório downshift-business-plan, se o texto mudar de novo.

