# Roadmap Unificado: Downshift Visual Suite (Svelte)

> **Discontinued 2026-10-29.** The dashboard commands were removed (N9); the dashboard code stays until W6.6. This roadmap stays as history.

> **Status:** In Progress  
> **Data:** 2026-10-22  
> **Stack:** Svelte 5, Vite, TailwindCSS, Chart.js  
> **Objetivo:** Criar um ecossistema visual completo para o Downshift em Svelte, com componentes reutilizáveis para ProgressBand, AgentsPanel e Dashboard.

## Decisão: Svelte-Only

O Claude Code usa Ink (React para terminal) nos mods, mas para ter uma experiência de desenvolvimento unificada e reutilizar componentes, vamos fazer **tudo em Svelte**:

1. **Dashboard SPA** — já scaffoldado, roda em `localhost:5173`
2. **ProgressBand** — componente Svelte (savvy-progress style)
3. **AgentsPanel** — componente Svelte (painel de agentes)
4. **Embed mode** — os componentes podem ser embedados em qualquer app web
5. **Claude Code mod** — wrapper que usa o Dashboard via iframe ou overlay

## Visão Geral

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                    Downshift Visual Suite (Svelte)                           │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│  ┌──────────────────────────────────────────────────────────────────────┐   │
│  │                    Svelte Component Library                           │   │
│  │  ┌────────────┐  ┌────────────┐  ┌────────────┐  ┌────────────┐     │   │
│  │  │ProgressBand│  │ AgentsPanel│  │ TokensDonut│  │ MetricCard │     │   │
│  │  └────────────┘  └────────────┘  └────────────┘  └────────────┘     │   │
│  │  ┌────────────┐  ┌────────────┐  ┌────────────┐  ┌────────────┐     │   │
│  │  │ AgentRow   │  │ Timeline   │  │ Heatmap    │  │ Compactor  │     │   │
│  │  └────────────┘  └────────────┘  └────────────┘  └────────────┘     │   │
│  └──────────────────────────────────────────────────────────────────────┘   │
│                              │                                               │
│              ┌───────────────┴───────────────┐                               │
│              │                               │                               │
│              ▼                               ▼                               │
│  ┌─────────────────────────┐     ┌─────────────────────────┐                │
│  │   Dashboard SPA         │     │   Embed Components      │                │
│  │   (full experience)     │     │   (any web app)         │                │
│  │   localhost:5173        │     │   npm package           │                │
│  └───────────┬─────────────┘     └───────────┬─────────────┘                │
│              │                               │                               │
│              └───────────────┬───────────────┘                               │
│                              │                                               │
│                              ▼                                               │
│              ┌───────────────────────────────┐                               │
│              │   downshift session --panel   │                               │
│              │   (unified JSON API)          │                               │
│              └───────────────────────────────┘                               │
│                              │                                               │
│                              ▼                                               │
│              ┌───────────────────────────────┐                               │
│              │   accounting v1               │                               │
│              │   (PriceBook + formula)       │                               │
│              └───────────────────────────────┘                               │
│                                                                              │
└─────────────────────────────────────────────────────────────────────────────┘
```

## Componentes Criados (✅ Done)

| Componente | Arquivo | Descrição |
|------------|---------|-----------|
| `ProgressBand` | `src/lib/components/ProgressBand.svelte` | Barra estilo savvy-progress |
| `AgentRow` | `src/lib/components/AgentRow.svelte` | Uma linha por agente |
| `AgentsPanel` | `src/lib/components/AgentsPanel.svelte` | Painel completo de agentes |
| `MetricCard` | `src/lib/components/MetricCard.svelte` | Card de métrica única |
| `TokensDonut` | `src/lib/components/TokensDonut.svelte` | Donut chart de tokens |
| `DecisionTimeline` | `src/lib/components/DecisionTimeline.svelte` | Timeline de decisões |
| `AgentHeatmap` | `src/lib/components/AgentHeatmap.svelte` | Heatmap por agent type |
| `CompactorStats` | `src/lib/components/CompactorStats.svelte` | Stats do Compactor |
| `Header` | `src/lib/components/Header.svelte` | Header do dashboard |

## Fases do Roadmap

### Fase 0: Accounting v1 no Panel (Foundation)

Integrar o PriceBook e formula no `session --panel` para que tanto o Panel quanto o Dashboard mostrem $ saved.

| ID | Tarefa | SP | Deps | Owner |
|----|--------|:--:|------|-------|
| A0.1 | Criar `EventsToTokenBuckets()` em `accounting/panel.go` | 2 | — | Backend |
| A0.2 | Chamar `formula.Saved()` no `sessionpanel.go` | 2 | A0.1 | Backend |
| A0.3 | Adicionar `cost_saved` struct no `panelOut` JSON | 1 | A0.2 | Backend |
| A0.4 | Testes: `sessionpanel_test.go` com cost_saved | 2 | A0.3 | Backend |
| A0.5 | Documentar caveat D4 (+15% bias) no output | 1 | A0.4 | Docs |
| **Subtotal** | | **8** | | |

### Fase 1: Downshift Panel (Claude Code mod)

Criar nosso próprio mod para Claude Code, substituindo o savvy-progress vendorizado.

| ID | Tarefa | SP | Deps | Owner |
|----|--------|:--:|------|-------|
| P1.1 | Scaffold: `mods/downshift-panel/.claude-plugin/plugin.json` | 1 | — | Frontend |
| P1.2 | Criar `hooks/hooks.json` e types | 1 | P1.1 | Frontend |
| P1.3 | Implementar atoms: `flow`, `agents`, `panel`, `now` | 2 | P1.2 | Frontend |
| P1.4 | Implementar `dsRefresh` (chama `downshift session --panel`) | 2 | P1.3 | Frontend |
| P1.5 | Implementar `dsKeyOf` (sha256 do tool_use_id) | 1 | P1.4 | Frontend |
| P1.6 | UI: Band component (barra acima do prompt) | 3 | P1.5 | Frontend |
| P1.7 | UI: AgentRow component (cada agente) | 3 | P1.6 | Frontend |
| P1.8 | UI: AgentsPanel component (painel `/agents-info`) | 3 | P1.7 | Frontend |
| P1.9 | UI: TokensSummary component (totais da sessão) | 2 | P1.8 | Frontend |
| P1.10 | UI: CostBadge component ($ saved, quando A0 pronto) | 2 | A0.3, P1.9 | Frontend |
| P1.11 | Integração: hooks `session.start`, `agent.spawn`, `turn.complete` | 2 | P1.10 | Frontend |
| P1.12 | Testes: `register.test.tsx`, `band.test.tsx` | 2 | P1.11 | Frontend |
| P1.13 | Brand: cores Downshift (black-first, green done) | 1 | P1.12 | Frontend |
| P1.14 | i18n: en, pt-BR (opcional: ru, he) | 1 | P1.13 | Frontend |
| P1.15 | Publicar no marketplace Downshift | 1 | P1.14 | Release |
| **Subtotal** | | **27** | | |

### Fase 2: Downshift Dashboard (Svelte SPA)

Dashboard web standalone para visualização rica (já scaffoldado).

| ID | Tarefa | SP | Deps | Owner |
|----|--------|:--:|------|-------|
| D2.1 | ✅ Scaffold: Vite + Svelte 5 + Tailwind | 2 | — | Done |
| D2.2 | ✅ Data Layer: types.ts, api.ts, transform.ts | 2 | D2.1 | Done |
| D2.3 | ✅ State Layer: session.svelte.ts | 2 | D2.2 | Done |
| D2.4 | ✅ UI: Header, MetricCard | 2 | D2.3 | Done |
| D2.5 | ✅ UI: TokensDonut (Chart.js) | 2 | D2.4 | Done |
| D2.6 | ✅ UI: DecisionTimeline | 2 | D2.5 | Done |
| D2.7 | ✅ UI: AgentHeatmap, CompactorStats | 2 | D2.6 | Done |
| D2.8 | Servidor proxy Go para dev (`/api/exec`) | 2 | D2.7 | Backend |
| D2.9 | CostBreakdown component (usa A0) | 2 | A0.3, D2.7 | Frontend |
| D2.10 | Historical chart (últimas 24h) | 3 | D2.9 | Frontend |
| D2.11 | Responsive e polish | 2 | D2.10 | Frontend |
| D2.12 | Build e dist estático | 1 | D2.11 | DevOps |
| **Subtotal** | | **24** | (14 done) | |

### Fase 3: Integração e Release

Unificar tudo e lançar.

| ID | Tarefa | SP | Deps | Owner |
|----|--------|:--:|------|-------|
| R3.1 | Comando `downshift dashboard` (abre o SPA) | 2 | D2.12 | Backend |
| R3.2 | Atualizar `downshift setup` para instalar Panel | 1 | P1.15 | Backend |
| R3.3 | Remover savvy-progress do marketplace | 1 | P1.15 | Release |
| R3.4 | Documentação: README, screenshots | 2 | R3.3 | Docs |
| R3.5 | Release v0.2.0 com Visual Suite | 1 | R3.4 | Release |
| **Subtotal** | | **7** | | |

## Resumo de Estimativas

| Fase | Nome | SP | Status |
|------|------|:--:|--------|
| 0 | Accounting v1 no Panel | 8 | 🔜 Next |
| 1 | Downshift Panel (mod) | 27 | 📋 Planned |
| 2 | Downshift Dashboard (SPA) | 24 | 🟡 14/24 done |
| 3 | Integração e Release | 7 | 📋 Planned |
| **Total** | | **66** | |

## Ordem de Execução (Loop Único)

```
┌─────────────────────────────────────────────────────────────────┐
│                      LOOP DE IMPLEMENTAÇÃO                       │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  Wave 1: Foundation (8 SP)                                       │
│  ├── A0.1 EventsToTokenBuckets                                   │
│  ├── A0.2 formula.Saved no panel                                 │
│  ├── A0.3 cost_saved no JSON                                     │
│  ├── A0.4 Testes                                                 │
│  └── A0.5 Docs                                                   │
│                                                                  │
│  Wave 2: Dashboard Completion (10 SP)                            │
│  ├── D2.8 Servidor proxy Go                                      │
│  ├── D2.9 CostBreakdown                                          │
│  ├── D2.10 Historical chart                                      │
│  ├── D2.11 Polish                                                │
│  └── D2.12 Build                                                 │
│                                                                  │
│  Wave 3: Panel Core (12 SP)                                      │
│  ├── P1.1-P1.5 Scaffold e data                                   │
│  ├── P1.6 Band                                                   │
│  ├── P1.7 AgentRow                                               │
│  └── P1.8 AgentsPanel                                            │
│                                                                  │
│  Wave 4: Panel Polish (15 SP)                                    │
│  ├── P1.9 TokensSummary                                          │
│  ├── P1.10 CostBadge                                             │
│  ├── P1.11 Hooks integration                                     │
│  ├── P1.12 Testes                                                │
│  ├── P1.13 Brand                                                 │
│  ├── P1.14 i18n                                                  │
│  └── P1.15 Publish                                               │
│                                                                  │
│  Wave 5: Release (7 SP)                                          │
│  ├── R3.1 CLI dashboard                                          │
│  ├── R3.2 Setup                                                  │
│  ├── R3.3 Remove savvy-progress                                  │
│  ├── R3.4 Docs                                                   │
│  └── R3.5 v0.2.0                                                 │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

## Arquitetura do Panel (Claude Code mod)

### Estrutura de arquivos

```
mods/downshift-panel/
├── .claude-plugin/
│   └── plugin.json           # name, version, userConfig
├── hooks/
│   ├── hooks.json            # { "modules": ["./register.tsx"] }
│   ├── register.tsx          # Entry point, atoms, hooks
│   ├── components/
│   │   ├── Band.tsx          # Barra acima do prompt
│   │   ├── AgentRow.tsx      # Uma linha por agente
│   │   ├── AgentsPanel.tsx   # Painel completo
│   │   ├── TokensSummary.tsx # Totais da sessão
│   │   └── CostBadge.tsx     # $ saved badge
│   ├── lib/
│   │   ├── downshift.ts      # dsRefresh, dsKeyOf
│   │   ├── format.ts         # formatTokens, formatUSD
│   │   └── i18n.ts           # Strings por idioma
│   └── tests/
│       ├── register.test.tsx
│       └── band.test.tsx
├── types/
│   └── index.d.ts
└── README.md
```

### Diferenças do savvy-progress

| Aspecto | savvy-progress | downshift-panel |
|---------|----------------|-----------------|
| **Origem** | Vendorizado (johnnyvizz) | Próprio |
| **Preços** | `PRICES` hardcoded | `session --panel` com PriceBook |
| **$ vs tokens** | $ estimate | Tokens (default) + $ (quando A0 pronto) |
| **Brand** | Patch em cima do upstream | Nativo Downshift |
| **i18n** | en, ru, he | en, pt-BR (core) |
| **Flow** | Suporta savvy-flow | Focado em subagents |
| **Maintenance** | Rebase em cada upstream | Evolução própria |

### Paleta de cores (brand tokens)

```typescript
const COLORS = {
  // Base
  BG: '#0a0a0a',        // brand black
  TEXT: '#f5f7fa',      // white
  MUTED: '#868A90',     // grey
  
  // States
  RUNNING: '#f5f7fa',   // white bar while running
  DONE: '#22c55e',      // green means done
  FAILED: '#ef4444',    // red for errors
  
  // Tiers (grey ramp by lightness)
  TIER_FRONTIER: '#FFFFFF',
  TIER_HEAVY: '#D6DAE2',
  TIER_MEDIUM: '#AAB0B8',
  TIER_SMALL: '#868A90',
  TIER_OTHER: '#5C6168',
  
  // Accent
  PILL_BG: '#f5f7fa',
  PILL_INK: '#0a0a0a',
};
```

## Decisões Técnicas

### Panel (Claude Code mod)

1. **React (Ink)** para UI no terminal (Claude Code usa Ink internamente)
2. **Atoms** para estado reativo (API do Claude Code)
3. **Web Crypto** para sha256 do tool_use_id
4. **$.process.run** para chamar `downshift session --panel`

### Dashboard (Svelte SPA)

1. **Svelte 5 Runes** para estado reativo (já implementado)
2. **Chart.js** para visualizações (já integrado)
3. **Proxy Go** para dev (chama CLI, devolve JSON)
4. **Static build** para produção (servido pelo CLI ou standalone)

### Shared

1. **JSON API unificada**: `downshift session --panel --json`
2. **PriceBook + formula** para cálculo de $
3. **Mesmo modelo de dados** em ambos os clientes

## Riscos e Mitigações

| Risco | Mitigação |
|-------|-----------|
| Claude Code API muda | Manter compatibilidade com 2.1.287+ |
| Panel fica pesado | Medir render time, lazy load |
| Dashboard sem proxy | Mock data para dev |
| $ calculation diverge | Mesma função `formula.Saved()` |
| i18n incompleto | en como fallback universal |

## Métricas de Sucesso

- [ ] Panel aparece no `/agents-info` do Claude Code
- [ ] Dashboard roda em `localhost:5173` com mock
- [ ] $ saved aparece em ambos quando A0 completo
- [ ] Testes passam: `go test ./...` e `vitest`
- [ ] Release v0.2.0 publicada com os dois

## Próximos passos

1. **Hoje:** Começar Wave 1 (A0.1-A0.5) — foundation
2. **Depois:** Wave 2 (D2.8-D2.12) — dashboard completion
3. **Depois:** Waves 3-4 (P1.x) — panel
4. **Final:** Wave 5 (R3.x) — release

---

## Checklist para Loop

```markdown
## Wave 1: Foundation (8 SP)
- [ ] A0.1 EventsToTokenBuckets
- [ ] A0.2 formula.Saved no panel
- [ ] A0.3 cost_saved no JSON
- [ ] A0.4 Testes
- [ ] A0.5 Docs

## Wave 2: Dashboard Completion (10 SP)
- [ ] D2.8 Servidor proxy Go
- [ ] D2.9 CostBreakdown
- [ ] D2.10 Historical chart
- [ ] D2.11 Polish
- [ ] D2.12 Build

## Wave 3: Panel Core (12 SP)
- [ ] P1.1-P1.5 Scaffold e data
- [ ] P1.6 Band
- [ ] P1.7 AgentRow
- [ ] P1.8 AgentsPanel

## Wave 4: Panel Polish (15 SP)
- [ ] P1.9 TokensSummary
- [ ] P1.10 CostBadge
- [ ] P1.11 Hooks integration
- [ ] P1.12 Testes
- [ ] P1.13 Brand
- [ ] P1.14 i18n
- [ ] P1.15 Publish

## Wave 5: Release (7 SP)
- [ ] R3.1 CLI dashboard
- [ ] R3.2 Setup
- [ ] R3.3 Remove savvy-progress
- [ ] R3.4 Docs
- [ ] R3.5 v0.2.0
```
