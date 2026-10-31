# Context diet

What every session loads, and spends tokens on, before the first message.

## Measure

```bash
wc -c CLAUDE.md AGENTS.md GEMINI.md .claude/rules/* .cursor/rules/* .kiro/steering/* \
  ~/.claude/CLAUDE.md ~/.codex/AGENTS.md 2>/dev/null | sort -rn
```

Add: the `description:` line of every installed `SKILL.md` (loaded in every session, used
in few); the tool schemas of every connected MCP server (often the largest item and the
least visible); rules added by portable compression skills. Bytes / 4 is an estimate.

## Move, do not delete

| Content | Belongs in |
|---|---|
| Rules for one language, framework or directory | a file-matched rule (`.claude/rules/*.md` with `paths`, `.cursor/rules/*.mdc` with globs) |
| A procedure used once a week | a skill with a one-line description and the detail in `references/` |
| Reference tables, catalogs, long examples | a doc the agent reads on demand, linked from the rule file |
| MCP servers used in one project | that project's config, not the user-level one |

Keep the always-loaded rule file at or below 200 lines (the Downshift repo applies this to
itself). Show the proposed moves as a list, apply them only after a yes, and re-measure.
