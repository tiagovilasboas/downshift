# Spike: Sessions-First UI

**Status:** 📋 Planned  
**Created:** 2026-10-22  
**Author:** Tiago Vilas Boas  
**Depends on:** PR #447 (merged)

## Objective

Reorganizar o dashboard para que **Sessions** sejam a entidade principal,
com agents aparecendo **dentro** de cada session (não como lista flat separada).

## Current State

```
┌─────────────────────────────────────┐
│ Tab: Sessions    Tab: Agents        │
├─────────────────────────────────────┤
│ Sessions (lista flat)               │
│ - session abc...  43 spawns         │
│ - session def...  9 spawns          │
│                                     │
│ Agents (lista flat separada)        │
│ - agent 1 (running)                 │
│ - agent 2 (finished) ← some em 1min │
└─────────────────────────────────────┘
```

## Proposed State

```
┌─────────────────────────────────────┐
│ Sessions                            │
├─────────────────────────────────────┤
│ ▼ Session abc... (claude-code)      │
│   │ 43 spawns · 15.9M tokens saved  │
│   │                                 │
│   ├─ Agent xyz (running) ●          │
│   ├─ Agent uvw (switched) ○         │
│   └─ +2 more active                 │
│                                     │
│ ▶ Session def... (codex)            │
│   │ 9 spawns · 0 tokens saved       │
└─────────────────────────────────────┘
```

## Model Mental

1. **Session** = uma conversa/tarefa (persiste no telemetry)
2. **Agents** = subagents spawned dentro dessa session
3. Agents finalizados somem após ~1min (não acumulam)
4. Agregados (switched, held, tokens_saved) persistem na session

## Tasks

| Task | SP | Description |
|------|----|-----------------------|
| T1: Remove Agents tab | 1 | Remover tab separada de agents |
| T2: Session expand/collapse | 2 | Click na session expande para mostrar agents |
| T3: Inline agents list | 2 | Agents aparecem dentro da session expandida |
| T4: Active indicator | 1 | Dot verde para agents ativos, cinza para finalizados |
| T5: Session summary | 1 | Linha de resumo: spawns, switched, tokens_saved |

**Total: 7 story points**

## API (sem breaking change)

A API atual já suporta isso:

```bash
# Lista sessions com agregados
GET /api/sessions → [{session_id, harness, spawn_count, tokens_saved}]

# Detalhe com spawns (agents)
GET /api/session/:id → {spawns: [...agents...]}
```

O frontend faz o expand client-side chamando `/api/session/:id` ao clicar.

## UI Components

```
SessionList.svelte
├── SessionRow.svelte (colapsado por default)
│   ├── HarnessIcon
│   ├── SessionSummary (spawns, tokens, time)
│   └── ExpandButton
└── AgentList.svelte (quando expandido)
    └── AgentRow.svelte
        ├── StatusDot (running/finished)
        ├── ModelSwitch (from → to)
        └── TokensSaved
```

## Acceptance Criteria

- [ ] Sessions são a entidade principal no dashboard
- [ ] Agents aparecem dentro de cada session (não em tab separada)
- [ ] Click na session expande/colapsa lista de agents
- [ ] Agents finalizados somem após ~1min
- [ ] Agregados (tokens_saved) persistem na session
- [ ] UI responsiva e fluida

## Out of Scope

- Mudanças na API backend
- Persistência de agents finalizados além do tail window
- Histórico de agents por session (além do telemetry atual)

## References

- PR #447: Exit mock mode
- PR #446: Dashboard simplification
- Telemetry: `internal/telemetry/events.go`
