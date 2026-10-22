# Spike: Dashboard Real Data (Exit Mock Mode)

**Status:** ✅ Complete  
**Created:** 2026-10-22  
**Author:** Tiago Vilas Boas  
**PR:** #447

## Objective

O dashboard estava bonito mas usava dados mock por padrão (`useMock = $state(true)`).
A API backend já lia dados reais do telemetry. Conectamos frontend ↔ backend.

## Problem Analysis

```
Frontend (Svelte)          Backend (Go)
─────────────────         ──────────────
useMock = true  ────X───  /api/sessions (real data!)
        ↓
useMock = false ────✓───  Real telemetry data!
```

## Roadmap

| Task | SP | Description | Status |
|------|----|-----------------------|--------|
| T1: Default to live | 1 | `useMock = $state(false)` | ✅ Done |
| T2: Remove mock toggle | 1 | Removido do UI e state | ✅ Done |
| T3: API URL auto-detect | 1 | `window.location.origin` quando servido pelo Go | ✅ Done |
| T4: Error handling | 1 | Erro claro quando API não responde | ✅ Done |
| T5: E2E validation | 1 | Dados reais verificados | ✅ Done |
| T6: Cleanup mock code | 1 | `mockPanel()` removido | ✅ Done |

**Total: 6 story points**

## Time

| Role | Agent | Responsibility |
|------|-------|----------------|
| Orchestrator | senior-staff-ai-finops | Coordenação, spike, roadmap |
| Implementer | senior-staff-fullstack | Frontend + API integration |

## Loop Engineer Cycle

```
SENSE   → ✅ Identificou useMock = true como problema
PLAN    → ✅ 6 tasks sequenciais
ACT     → ✅ Implementou todas as tasks
VERIFY  → ✅ Build + curl + dados reais
COMMIT  → ✅ Commit com evidência
REVIEW  → ✅ PR #447 aberto
```

## Acceptance Criteria

- [x] Dashboard inicia em modo live (não mock)
- [x] API URL detectada automaticamente quando servido pelo Go
- [x] Dados reais do telemetry aparecem no dashboard
- [x] Compactor stats reais do execwrap
- [x] Erro claro quando API não responde
- [x] Código mock removido

## Evidence

```bash
curl -s "http://localhost:1337/api/session/cbf6de37" | jq
```

```json
{
  "session": "cbf6de37",
  "switched": 7,
  "held": 13,
  "tokens_saved_estimate": 15909196,
  "spawns_count": 43,
  "compactor": {
    "commands": 497,
    "compacted": 42,
    "saved_bytes": 29754,
    "estimated_tokens_saved": 7438
  }
}
```

## Files Changed

```
dashboard/src/shared/data/api.ts          # Auto-detect URL, remove mockPanel()
dashboard/src/shared/state/session.svelte.ts  # Remove useMock, simplify
docs/spikes/dashboard-exit-mock.md        # This spike
```
