# Spike: Dashboard UI Simplification

**Status:** ✅ Complete  
**Created:** 2026-10-22  
**Author:** Tiago Vilas Boas  
**Commit:** b5e70fab

## Objective

Simplificar o dashboard do Downshift para uma estética dev tool minimalista:
- Lucide icons (open source, stroke-based)
- Brand tokens oficiais (zinc + verde accent sutil)
- Harness cards clicáveis → navegam para Sessions filtrado
- Sem animações complexas

## Roadmap

| Phase | Story Points | Description | Status |
|-------|--------------|-------------|--------|
| P1: Brand tokens | 1 | Usar `docs/brand/downshift-brand-tokens.css` | ✅ Done |
| P2: Logo oficial | 1 | Usar `docs/brand/downshift-mark-darkbg.svg` | ✅ Done |
| P3: Lucide icons | 2 | Substituir SVG custom por Lucide | ✅ Done |
| P4: HarnessList clicável | 2 | Cards navegam para Sessions com filtro | ✅ Done |
| P5: Simplificar agents | 2 | Remover animações, LiveAgent/AgentsList clean | ✅ Done |
| P6: E2E validation | 2 | API + build verification | ✅ Done |
| P7: Cleanup | 1 | Remover SVG animados não usados | ✅ Done |

**Total: 11 story points**

## Time

| Role | Agent | Responsibility |
|------|-------|----------------|
| Orchestrator | senior-staff-ai-finops | Coordenação, spike, roadmap |
| Implementer | senior-staff-fullstack | Frontend Svelte, CSS, icons |

## Loop Engineer Cycle

```
SENSE → PLAN → ACT → VERIFY → COMMIT → ✅
```

## Acceptance Criteria

- [x] Logo oficial do Downshift visível no header
- [x] Brand tokens aplicados (cores oficiais)
- [x] Harness cards listados com Lucide icons
- [x] Clicar harness → tab Sessions + filtro ativo
- [x] Verde aparece apenas como accent sutil (2 occurrences in CSS)
- [x] Sem animações complexas (só spinner básico)
- [x] Build passa sem erros (0 errors, 0 warnings)

## Verification Results

```bash
# API responds
curl http://localhost:1337/api/sessions
→ {"harnesses": ["codex", "claude-code"], "session_count": 3}

# Brand tokens in CSS
#0A0A0A (bg), #1A1A1A (surface), #22C55E (accent) ✓

# Logo mark found in bundle
"M28 22 H104" ✓

# Lucide loaded
"lucide" ✓

# Green usage: 2 occurrences (minimal, accent only) ✓
```

## Files Changed

```
dashboard/src/
├── app.css                          # Brand tokens oficiais
├── App.svelte                       # Logo + navegação
├── package.json                     # +lucide-svelte
└── features/agents/
    ├── AgentsList.svelte            # Simplificado
    ├── LiveAgent.svelte             # Sem AnimatedAgent
    ├── HarnessList.svelte           # NEW: Lucide + clicável
    └── index.ts                     # Exports atualizados
```

## Files Removed

```
dashboard/src/features/agents/
├── AnimatedAgent.svelte             # Não usado
└── characters/                      # Pasta inteira removida
    ├── Ghost.svelte
    ├── Robot.svelte
    ├── Cursor.svelte
    ├── Scroll.svelte
    ├── Rocket.svelte
    ├── Lock.svelte
    └── index.ts
```

## Notes

- Verde (#22C55E) é identidade do Downshift, usado com moderação
- Lucide icons são MIT, mesma lib do shadcn/ui
- O clique nos harness cards é o principal UX improvement
- Bundle size reduced: 188KB JS (was 200KB with animations)
