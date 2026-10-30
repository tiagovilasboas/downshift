# Session State

## Objetivo Atual
- Implementar o registro declarativo do MCP server `downshift` em formato JSON (`mcpServers`) para Cursor, Google Antigravity e Kiro, com suporte a merge atômico, nil-safety e rollback idempotente (`--undo`).

## Status em Tempo Real
- **Fase Atual:** Implementação conduzida pela equipe Senior Staff (AI FinOps, Fullstack e QA) com veredito PASS. Branch `feat/mcp-registration-json` pronta para commit e PR.
- **Branch atual:** `feat/mcp-registration-json`
- **Bloqueios:** Nenhum.

## Decisões Recentes
- Targets MCP JSON: Cursor (`~/.cursor/mcp.json`), Antigravity (`~/.gemini/config/mcp_config.json`) e Kiro (`~/.kiro/settings/mcp.json`).
- Isolamento com marcador de segurança: `env.DOWNSHIFT_MANAGED = "setup"`.
- Resolução e preservação de symlinks via `filepath.EvalSymlinks`.
- Nil-safety rigorosa em todos os streams de I/O (`o.In`, `o.Out`, `o.Err`).
