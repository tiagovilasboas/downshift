---
name: Plan compatibility / rewrite-honored report
about: Report whether a harness plan/build actually honors or drops model rewrites
title: 'compat: '
labels: compatibility, harness
---

## Harness & Environment

- **Harness:** [ ] Claude Code / [ ] Cursor / [ ] Codex / [ ] Antigravity / [ ] KiroCrew / [ ] Grok / [ ] Other
- **Harness Version / Build:** (e.g. Cursor 0.45.x, Claude Code 0.2.x)
- **Account / Plan Tier:** (e.g. Free, Pro legacy, Pro/Ultra usage-based, Teams, API)
- **OS / Architecture:** (e.g. macOS arm64, Linux x86_64)

## Pre-conditions & Configuration

- [ ] `downshift` version (`downshift --version` or build commit):
- [ ] Session allowlist configured (if applicable, e.g. `~/.harness-downshift/session-models.json`):
- [ ] Hook configuration confirmed active in harness settings

## Observed Behavior

### 1. Emission (Hook Output)
<!-- Run or inspect hook output: downshift try "<prompt>" <harness> or hook stderr -->
```
PASTE HOOK OUTPUT OR REWRITE EMITTED EVENT HERE
```

### 2. Execution (Spawned Child Model)
<!-- Did the child subagent actually run with the rewritten model? Check UI model badge, process flags, or usage logs -->
- **Did the child spawn with the rewritten model?**
  - [ ] Yes (rewrite honored)
  - [ ] No (silently dropped / fell back to parent model)
  - [ ] Blocked / Error
- **Evidence / Logs:**
```
PASTE EVIDENCE (stderr, debug log, UI inspect, or post-tool usage event)
```

## Matrix update recommendation

- [ ] Updates an existing row in `docs/harness-matrix.md`
- [ ] Proposes a new harness / plan combination
- **Suggested row entry:**
