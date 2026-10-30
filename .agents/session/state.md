# Session State

## Objetivo Atual
- Implementar a evolução do Setup Wizard do Downshift (cards S-W1 a S-W5), os 6 playbooks de engenharia com state-machines em Mermaid e a Universal Skill, mantendo o gate `make check` verde e entrega contínua.

## Status em Tempo Real
- **Fase Atual:** Implementação dos cards S-W3 (Playbooks & Universal Skill) e S-W1/S-W2 (TTY Detection & Setup Wizard TUI).
- **Branch atual:** `main` (sincronizada com `origin/main` após PRs #618, #619, #620, #621).
- **Bloqueios:** Nenhum. Força total ativada pelo mantenedor.

## Decisões Recentes
- O8 mantido: orquestrador escolhe, Downshift aplica na política declarada.
- 7 harnesses ordenados por adoção: Claude Code, Cursor, OpenCode, Codex, Antigravity, Kiro, KiroCrew.
- 6 playbooks de engenharia canônicos com diagramas Mermaid de loop e circuit breaker.
