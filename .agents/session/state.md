# Session State

## Objetivo Atual
- Incremento 2 (Opção B - 3 SP): R8a: `downshift policy apply --harness claude-code [--dry-run] [--yes]`.
- Branch de trabalho: `feat/policy-apply-claude`.

## Status em Tempo Real
- **Fase Atual:** Implementação de `policy apply`, suite de testes unitários (AppSec, symlinks, dry-run, backups) e documentação concluídas com sucesso. Validação integral com `make check` 100% verde (Host, Windows, Linux) e aprovação formal de QA (PASS).
- **Branch atual:** `feat/policy-apply-claude`
- **Bloqueios:** Nenhum.

## Entregas Recentes Concluídas
- **Incremento 2 (Opção B - 3 SP):** Implementação de `downshift policy apply --harness claude-code` compilando perfis declarativos da política (`policy.json`) em `.claude/agents/<name>.md`. Resolução e preservação de symlinks, sanitização contra path traversal, backups prévios atômicos (`0600`), suporte completo a `--dry-run` e confirmação interativa. Arquivos enxutos (< 300 linhas cada).
- **PR #627 (Incremento 1 - 2 SP):** Claude Effort Optimizer (`cc-plugin-effort-optimizer@builtin`) adicionado como default recomendado ativo por padrão para Claude Code no `downshift setup`, com suporte a `--no-session-policy`, preservação de configurações prévias do usuário, exibição no wizard TUI e nos relatórios do `doctor`.
- **PR #626:** Registro declarativo JSON para Cursor (`~/.cursor/mcp.json`), Antigravity (`~/.gemini/config/mcp_config.json`) e Kiro (`~/.kiro/settings/mcp.json`). Preservação de symlinks, nil-safety total e reversão limpa com `--undo`.
