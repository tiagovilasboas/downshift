# Spike: Downshift Dashboard (Svelte)

> **Status:** Spike  
> **Data:** 2026-10-22  
> **Stack:** Svelte 5, Vite, TailwindCSS  
> **Princípios:** SRP, Clean Code, Dependency Rule, KISS

## Objetivo

Dashboard web para visualizar em tempo real o que o Downshift está economizando:
tokens, custos, decisões de roteamento e performance do Compactor.

## Proposta de valor

| Funcionalidade | Valor | Dados |
|----------------|-------|-------|
| **Live Session** | Ver economia em tempo real enquanto trabalha | `session --panel` |
| **Cost Breakdown** | Entender onde o $ está sendo economizado | PriceBook + formula |
| **Decision Timeline** | Auditar cada decisão de roteamento | `spawns[]` |
| **Agent Heatmap** | Identificar quais agent types mais economizam | `by_agent_type[]` |
| **Compactor Stats** | Medir impacto da compactação de output | `compactor` |
| **Historical Trends** | Ver evolução ao longo do tempo | `downshift stats` |

## Arquitetura

```
┌─────────────────────────────────────────────────────────────────┐
│                         Dashboard (Svelte)                       │
├─────────────────────────────────────────────────────────────────┤
│  UI Layer                                                        │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐           │
│  │ Header   │ │ Metrics  │ │ Timeline │ │ Heatmap  │           │
│  │ .svelte  │ │ .svelte  │ │ .svelte  │ │ .svelte  │           │
│  └────┬─────┘ └────┬─────┘ └────┬─────┘ └────┬─────┘           │
│       │            │            │            │                   │
├───────┴────────────┴────────────┴────────────┴──────────────────┤
│  State Layer (Svelte 5 Runes)                                    │
│  ┌─────────────────────────────────────────────────────────────┐│
│  │ session.svelte.ts    │ Reactive state para dados da sessão  ││
│  │ preferences.svelte.ts│ Config do usuário (theme, refresh)   ││
│  └─────────────────────────────────────────────────────────────┘│
├─────────────────────────────────────────────────────────────────┤
│  Data Layer (puro, sem framework)                                │
│  ┌─────────────────────────────────────────────────────────────┐│
│  │ types.ts        │ Tipos TypeScript dos dados               ││
│  │ api.ts          │ Fetch do CLI (shell ou websocket)        ││
│  │ transform.ts    │ Transformações de dados para UI          ││
│  └─────────────────────────────────────────────────────────────┘│
└─────────────────────────────────────────────────────────────────┘
         │
         ▼
┌─────────────────────────────────────────────────────────────────┐
│                    Downshift CLI                                 │
│  downshift session --panel --json                                │
│  downshift stats --json                                          │
│  downshift report --json                                         │
└─────────────────────────────────────────────────────────────────┘
```

### Dependency Rule

```
UI → State → Data → CLI
 ↓      ↓      ↓
Svelte  Runes  Pure TS  (framework-free)
```

- **Data Layer** não importa Svelte, não conhece UI
- **State Layer** importa Data, não conhece componentes específicos
- **UI Layer** importa State, renderiza

## Estrutura de arquivos

```
dashboard/
├── src/
│   ├── lib/
│   │   ├── data/           # Data Layer (puro)
│   │   │   ├── types.ts
│   │   │   ├── api.ts
│   │   │   └── transform.ts
│   │   ├── state/          # State Layer (runes)
│   │   │   ├── session.svelte.ts
│   │   │   └── preferences.svelte.ts
│   │   └── components/     # UI Layer
│   │       ├── Header.svelte
│   │       ├── MetricsCard.svelte
│   │       ├── TokensDonut.svelte
│   │       ├── CostBreakdown.svelte
│   │       ├── DecisionTimeline.svelte
│   │       ├── AgentHeatmap.svelte
│   │       └── CompactorStats.svelte
│   ├── routes/
│   │   └── +page.svelte    # Layout principal
│   └── app.css             # Tailwind imports
├── static/
├── svelte.config.js
├── vite.config.ts
├── tailwind.config.js
└── package.json
```

## Componentes principais

### 1. Header

```svelte
<!-- Sessão atual, status de conexão, refresh rate -->
<header>
  <h1>Downshift</h1>
  <span class="session-id">{session.slice(0,8)}</span>
  <StatusDot connected={isConnected} />
  <RefreshSelector bind:interval />
</header>
```

### 2. MetricsCard (SRP: um número, uma métrica)

```svelte
<script lang="ts">
  interface Props {
    label: string;
    value: number | string;
    delta?: number;
    format?: 'number' | 'tokens' | 'currency' | 'percent';
  }
  let { label, value, delta, format = 'number' }: Props = $props();
</script>

<div class="metric-card">
  <span class="label">{label}</span>
  <span class="value">{formatValue(value, format)}</span>
  {#if delta}
    <span class="delta" class:positive={delta > 0}>{delta}%</span>
  {/if}
</div>
```

### 3. TokensDonut (visualização principal)

```svelte
<!-- Donut chart: work vs cache, saved vs total -->
<TokensDonut 
  workTokens={tokens.work_tokens}
  workSaved={tokens.work_saved}
  cacheTokens={tokens.cache_tokens}
  cacheSaved={tokens.cache_saved}
/>
```

### 4. CostBreakdown (quando tiver $ integrado)

```svelte
<!-- Breakdown do custo: actual vs baseline -->
<CostBreakdown
  actualCost={cost.actual_cost_usd}
  baselineCost={cost.baseline_cost_usd}
  savedUsd={cost.saved_usd}
  quality={cost.quality}
/>
```

### 5. DecisionTimeline

```svelte
<!-- Lista de spawns com estado e modelo -->
{#each spawns as spawn}
  <TimelineItem 
    state={spawn.state}
    from={spawn.from_model}
    to={spawn.to_model}
    tokens={spawn.tokens_label}
  />
{/each}
```

### 6. AgentHeatmap

```svelte
<!-- Grid de agent types com intensidade por economia -->
<AgentHeatmap types={byAgentType} />
```

### 7. CompactorStats

```svelte
<!-- Estatísticas do Compactor -->
<CompactorStats
  commands={compactor.commands}
  compacted={compactor.compacted}
  savedBytes={compactor.saved_bytes}
  estimatedTokens={compactor.estimated_tokens_saved}
/>
```

## Data Layer (types.ts)

```typescript
// Espelha o JSON do CLI, sem lógica de UI

export interface PanelView {
  session: string;
  switched: number;
  model_switched: number;
  effort_changed: number;
  held: number;
  held_guardrail: number;
  upshift_not_applied: number;
  observed: number;
  tokens: TokenSaving;
  tokens_line: string;
  by_agent_type: AgentType[];
  spawns: Spawn[];
}

export interface TokenSaving {
  children: number;
  saved_children: number;
  work_tokens: number;
  work_saved: number;
  cache_tokens: number;
  cache_saved: number;
}

export interface Spawn {
  tool_use: string;
  state: 'switched' | 'held' | 'paused' | 'control' | 'blocked';
  from_model: string;
  to_model: string;
  observed_model?: string;
  label: string;
  tokens: TokenSaving;
  tokens_label: string;
}

export interface Compactor {
  scope: string;
  commands: number;
  compacted: number;
  raw_bytes: number;
  saved_bytes: number;
  estimated_tokens_saved: number;
}

export interface CostSaved {
  actual_cost_usd: number;
  baseline_cost_usd: number;
  saved_usd: number;
  saved_percent: number;
  quality: 'exact' | 'mixed' | 'partial' | 'unpriced';
}

export interface PanelOut extends PanelView {
  compactor?: Compactor;
  tokens_saved_estimate: number;
  cost_saved?: CostSaved; // future: quando T1-T6 estiver pronto
}
```

## Data Layer (api.ts)

```typescript
// Busca dados do CLI, agnóstico de framework

export async function fetchPanel(sessionId?: string): Promise<PanelOut> {
  const args = sessionId ? `--session=${sessionId}` : '';
  const result = await execCommand(`downshift session --panel ${args} --json`);
  return JSON.parse(result);
}

export async function fetchStats(): Promise<StatsOut> {
  const result = await execCommand('downshift stats --json');
  return JSON.parse(result);
}

// Abstração para ambiente (Tauri, Electron, ou dev server com proxy)
async function execCommand(cmd: string): Promise<string> {
  if (window.__TAURI__) {
    return await invoke('exec_command', { cmd });
  }
  // Dev: proxy para o servidor local
  const res = await fetch('/api/exec', {
    method: 'POST',
    body: JSON.stringify({ cmd }),
  });
  return res.text();
}
```

## State Layer (session.svelte.ts)

```typescript
// Svelte 5 runes para estado reativo

import { fetchPanel } from '$lib/data/api';
import type { PanelOut } from '$lib/data/types';

// Estado reativo
let panel = $state<PanelOut | null>(null);
let error = $state<string | null>(null);
let loading = $state(false);
let lastUpdate = $state<Date | null>(null);

// Derived values
const tokensSavedPercent = $derived(
  panel?.tokens.work_tokens 
    ? (panel.tokens.work_saved / panel.tokens.work_tokens) * 100 
    : 0
);

const totalSaved = $derived(
  panel ? panel.tokens.work_saved + panel.tokens.cache_saved : 0
);

// Actions
export async function refresh(sessionId?: string) {
  loading = true;
  error = null;
  try {
    panel = await fetchPanel(sessionId);
    lastUpdate = new Date();
  } catch (e) {
    error = e instanceof Error ? e.message : 'Unknown error';
  } finally {
    loading = false;
  }
}

// Auto-refresh
let intervalId: number | null = null;

export function startAutoRefresh(intervalMs: number = 3000) {
  stopAutoRefresh();
  refresh(); // initial
  intervalId = setInterval(refresh, intervalMs);
}

export function stopAutoRefresh() {
  if (intervalId) {
    clearInterval(intervalId);
    intervalId = null;
  }
}

// Export do state
export { panel, error, loading, lastUpdate, tokensSavedPercent, totalSaved };
```

## Layout principal (+page.svelte)

```svelte
<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { 
    panel, loading, error, lastUpdate,
    startAutoRefresh, stopAutoRefresh 
  } from '$lib/state/session.svelte';
  
  import Header from '$lib/components/Header.svelte';
  import MetricsCard from '$lib/components/MetricsCard.svelte';
  import TokensDonut from '$lib/components/TokensDonut.svelte';
  import DecisionTimeline from '$lib/components/DecisionTimeline.svelte';
  import AgentHeatmap from '$lib/components/AgentHeatmap.svelte';
  import CompactorStats from '$lib/components/CompactorStats.svelte';

  onMount(() => startAutoRefresh(3000));
  onDestroy(() => stopAutoRefresh());
</script>

<div class="dashboard">
  <Header session={panel?.session} {loading} {lastUpdate} />
  
  {#if error}
    <div class="error">{error}</div>
  {:else if panel}
    <!-- Metrics row -->
    <div class="metrics-grid">
      <MetricsCard 
        label="Tokens Saved" 
        value={panel.tokens_saved_estimate} 
        format="tokens" 
      />
      <MetricsCard 
        label="Switched" 
        value={panel.switched} 
      />
      <MetricsCard 
        label="Held" 
        value={panel.held} 
      />
      <MetricsCard 
        label="Work Saved" 
        value={panel.tokens.work_saved / panel.tokens.work_tokens * 100} 
        format="percent" 
      />
    </div>
    
    <!-- Main content -->
    <div class="main-grid">
      <TokensDonut tokens={panel.tokens} />
      <DecisionTimeline spawns={panel.spawns} />
    </div>
    
    <!-- Secondary -->
    <div class="secondary-grid">
      <AgentHeatmap types={panel.by_agent_type} />
      {#if panel.compactor}
        <CompactorStats compactor={panel.compactor} />
      {/if}
    </div>
  {:else}
    <div class="loading">Loading...</div>
  {/if}
</div>

<style>
  .dashboard {
    @apply min-h-screen bg-gray-900 text-gray-100 p-6;
  }
  .metrics-grid {
    @apply grid grid-cols-4 gap-4 mb-6;
  }
  .main-grid {
    @apply grid grid-cols-2 gap-6 mb-6;
  }
  .secondary-grid {
    @apply grid grid-cols-2 gap-6;
  }
</style>
```

## Paleta de cores (dark theme)

```css
:root {
  --bg: #0f172a;           /* slate-900 */
  --bg-card: #1e293b;      /* slate-800 */
  --text: #f1f5f9;         /* slate-100 */
  --text-muted: #94a3b8;   /* slate-400 */
  --accent: #8b5cf6;       /* violet-500 - cor do Downshift */
  --success: #22c55e;      /* green-500 */
  --warning: #f59e0b;      /* amber-500 */
  --danger: #ef4444;       /* red-500 */
  
  /* Estados de decisão */
  --switched: #8b5cf6;     /* violet */
  --held: #94a3b8;         /* gray */
  --upshift: #f59e0b;      /* amber */
  --blocked: #ef4444;      /* red */
}
```

## Roadmap de implementação

| Fase | Tarefas | SP |
|------|---------|:--:|
| **P0: Scaffold** | Vite + Svelte 5 + Tailwind, estrutura de pastas | 2 |
| **P1: Data Layer** | types.ts, api.ts com mock | 2 |
| **P2: State Layer** | session.svelte.ts com refresh | 2 |
| **P3: UI Core** | Header, MetricsCard, layout | 3 |
| **P4: Visualizações** | TokensDonut, DecisionTimeline | 3 |
| **P5: Secondary** | AgentHeatmap, CompactorStats | 2 |
| **P6: Polish** | Animações, responsive, error states | 2 |
| **Total** | | **16** |

## Decisões técnicas

1. **Svelte 5 Runes** em vez de stores: API mais simples, menos boilerplate
2. **Vite** em vez de SvelteKit: não precisa de SSR, é um SPA local
3. **Tailwind** para styling: rápido, consistente, dark mode nativo
4. **Chart.js ou D3** para visualizações: TBD na P4
5. **Polling 3s** inicial: websocket é over-engineering para MVP

## Fora de escopo (MVP)

- [ ] Histórico persistente (usar `downshift stats` no CLI)
- [ ] Múltiplas sessões simultâneas
- [ ] Export de relatórios
- [ ] Temas customizáveis
- [ ] Notificações/alertas
- [ ] Mobile responsive (desktop-first)
