# Session State

- **Branch Atual:** `downshift/mvp-ponto-zero`
- **Fase Atual:** Onda 1.5 (Setup Completo & Release Readiness — Business Plan W7)
- **Status:** Roadmap e Ondas alinhados com o Business Plan (`downshift-business-plan`).
- **Próximo Objetivo:** Implementar S1 (ownership markers) e S2 (skill em ~/.claude/skills). Pendências: `setup --undo` não remove o policy.json criado; nit do revisor sobre o literal `claude-code` duplicado.
- **Bloqueios:** Nenhum
- **Commits na Branch:**
  - `7a9becc1`: feat(cli): add smart zero-prompt default entry when invoked without args (F0.1)
  - `73fe2981`: feat(setup): add downshift uninstall command with backup restoration and cache purge (F0.2)
  - `78b32c06`: feat(setup): add interactive terminal UI setup via --tui flag (F0.3)
  - `b10d75d0`: feat(cascade): implement episodic failure tracker and dead-ends sensor (F1.1)
  - `f817ac5c`: feat(routing): add sensor-guided auto-upgrade cascade to pre-tool-use decider (F1.2)
  - `b9919bbf`: feat(telemetry): add git acceptance detection and accept command for used and discarded code (F1.3)
  - `aa188066`: test(e2e): add end-to-end battery test for claude-code mvp lifecycle (F1.4)
  - `7de4a14f`: docs(coverage): inventory internal/cascade package in architecture coverage matrix (F1.5)
  - `cfb96ded`: feat(loop): implement autonomous engineering loop command with sensor check, cascade auto-upgrade and acceptance (F1.6)
  - `dce5db45`: feat(setup): auto-discover and seed relative model tiers in session-models.json for detected harnesses (F0.4)
  - `603725d0`: feat(uninstall): enhance --purge to completely remove ~/.downshift state dir and harness backup files
  - `aa2a24c1`: docs(help): clarify --purge in downshift uninstall help topic
  - `8af04fc7`: chore(session): update session tracking for uninstall purge feature
  - `4fc254ed`: fix(cascade): add inter-process directory lock and atomic rename to dead_ends, pass active model in loop failure
  - `b79162eb`: feat(setup): configure CLAUDE_CODE_SUBAGENT_MODEL_FORCE in claude-code settings env
  - `cda89de8`: test(claudecode): clear subagent model force env in session model test
  - `532bd279`: feat(routing): add default profile table for claude code built-in agent types (W7.1)
  - `50f80a0a`: feat(setup): add policy init step with dry run and yes gate (W7.2)
  - `fffde0a3`: fix(routing): keep declared profiles and hint tiers ahead of the default table
  - `9c1a3e62`: feat(routing): add economy mode as policy data (W7.9)

## Estado em 2026-10-28 (O8)
- Decisão do dono: o LLM orquestrador escolhe agente, provedor, modelo e esforço (como o caminho MCP do Alethe e o padrão Orchestrator-Workers da Anthropic). O Downshift aplica dentro da política declarada e mede. Removidos: tabela N11, escalonamento automático após falha, veto de risco embutido, escrita de `CLAUDE_CODE_SUBAGENT_MODEL_FORCE` no setup.
- Novo: playbooks de regras de trabalho (`internal/playbook`, `downshift playbook`) e skill `downshift-orchestrator`. O servidor MCP (`downshift mcp serve`) está implementado (ferramentas de informação, SDK oficial, duas eras do protocolo) e `downshift setup --mcp` registra no Claude Code e no Codex; as ferramentas que lançam workers seguem especificadas.

