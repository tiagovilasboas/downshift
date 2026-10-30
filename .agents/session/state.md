# Session State

## Objetivo Atual
- Implementar a evolução do Setup Wizard do Downshift (cards S-W1 a S-W5), os 6 playbooks de engenharia com state-machines em Mermaid e a Universal Skill, mantendo o gate `make check` verde e entrega contínua.

## Status em Tempo Real
- **Fase Atual:** S-W3 mesclado no PR #622. S-W1 e S-W2 mesclados no PR #623. S-W4 (Live Adapter Proof & Smoke Check) implementado e verificado com `make check` 100% verde.
- **Branch atual:** `feat/setup-live-smoke-check` pronta para commit e PR.
- **Bloqueios:** Nenhum. Força total ativada pelo mantenedor.

## Decisões Recentes
- O8 mantido: orquestrador escolhe, Downshift aplica na política declarada.
- 7 harnesses ordenados por adoção: Claude Code, Cursor, OpenCode, Codex, Antigravity, Kiro, KiroCrew.
- 6 playbooks de engenharia canônicos com diagramas Mermaid de loop e circuit breaker integrados no binary asset pool.
- Setup auto-detecta TTY para acionar o Wizard interativo passo-a-passo e suporta `--no-tui` / `--yes` / `--dry-run` para silent execution.
- Smoke handshake in-process (<50ms) valida adapters registrados em tempo real após a gravação da configuração.
