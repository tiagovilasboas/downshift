<!-- Copyright (c) 2026 Tiago Vilas Boas. SPDX-License-Identifier: Apache-2.0 -->

# Registering `downshift mcp serve` in each harness

Downshift by Tiago Vilas Boas · https://github.com/tiagovilasboas/downshift

Status: **Research, 2026-10-28**, input to card K6 (`setup --mcp`) and to the spec's open item G-MCP1. Evidence levels:
**docs** (official documentation), **local** (read-only `--help` or file structure on this machine), **binary**
(strings of the installed binary), **unverified**. No add or remove command was executed, and no server was started.

## Findings that shape the code

1. **Claude Code speaks both eras.** Its client has two runtimes: v1 (initialize handshake up to 2025-11-25) and v2
   (adds 2026-07-28), chosen by feature flags and version (docs, binary). A server that answers both eras, as
   `internal/mcp/transport` does, is the right shape.
2. **Tool names.** The model sees `mcp__<server>__<tool>`; the installed binary replaces characters outside
   `[a-zA-Z0-9_-]` with `_` (binary; docs state the same rule for plugin names). Our tools are therefore named by a
   plain verb (`providers`, `playbook`, `status`, `apply`) under the server name `downshift`, using `[a-z0-9_]` only.
3. **Claude Code user scope is `~/.claude.json`**, which Claude Code itself rewrites while sessions run. Write it
   through `claude mcp add -s user downshift -- <abs path to downshift> mcp serve`, not by editing the file (docs,
   local). Detect an existing entry by reading the JSON; `claude mcp list` and `get` start the servers.
4. **Formats differ**, so one adapter per harness, driven by data in the setup target:

| Harness | File | Root key | Entry | Comments |
|---|---|---|---|---|
| Claude Code | `~/.claude.json` (user) | `mcpServers` | `type` `stdio`, `command`, `args`, `env` | no |
| Codex | `~/.codex/config.toml` | `[mcp_servers.<name>]` | `command`, `args`, `[…env]` | yes (TOML) |
| Cursor | `~/.cursor/mcp.json` | `mcpServers` | `command`, `args`, `env` | not documented |
| OpenCode | `~/.config/opencode/opencode.json` or `.jsonc` | `mcp` | `type` `local`, `command` is one array, `environment` | yes (JSONC) |
| Antigravity | `~/.gemini/config/mcp_config.json` | `mcpServers` | `command`, `args`, `env` | no |
| Kiro | `~/.kiro/settings/mcp.json` | `mcpServers` | `command`, `args`, `env`, `autoApprove`, `disabled` | no |

5. **Safe writes.** Claude Code: delegate to the CLI. Codex: edit only the `[mcp_servers.downshift]` block as text
   (a TOML library would drop comments and reorder keys). OpenCode: text-aware editing, and both `.json` and `.jsonc`
   may exist. Antigravity: `~/.gemini/antigravity/mcp_config.json` is a symlink to `~/.gemini/config/mcp_config.json`;
   resolve it and write the real target atomically (a rename onto the link path replaces the link). Cursor, Kiro and
   Antigravity: strict JSON, merge into the existing object, atomic write, keep the file mode. Kiro keeps a `mcp.lock`
   beside its file; prefer `kiro-cli mcp add --force`. KiroCrew's own agents set `includeMcpJson:false` and do not
   read the global file.
6. **Approval.** Claude Code asks nothing for user and local scope; a project `.mcp.json` asks in interactive
   sessions and is the wrong place for an absolute home path. Codex has per-server `default_tools_approval_mode`.
   Cursor asks before using an MCP tool. Kiro lists tools in `autoApprove`; empty means ask each time.

## Scope of K6 in the MVP

Claude Code (through its CLI) and Codex (text block) in K6; the other four are stage 5, with the adapters above as
their starting point. Every write is opt-in (`setup --mcp`), shows a dry run first, backs up, is marked, and is undone
by `setup --undo` without touching the user's other servers.

## Unverified

Whether the tool-name rule applies to dots in tool names (inferred from the binary only; our names avoid dots); what
`claude mcp add` does on a duplicate name; where `.claude.json` lives under `CLAUDE_CONFIG_DIR`; whether Codex's
`mcp add` preserves comments; the tool-naming and protocol revision of Codex, Cursor, OpenCode, Antigravity and Kiro;
whether Cursor tolerates comments in `mcp.json`; the `includeMcpJson` semantics of Kiro agents.
