# Downshift Dashboard

Visual dashboard for monitoring Downshift token savings in real-time.

## Stack

- **Svelte 5** with runes for reactive state
- **Vite** for fast dev experience
- **TailwindCSS** for styling
- **Chart.js** for visualizations

## Architecture

```
UI Layer (Svelte components)
    ↓
State Layer (Svelte 5 runes)
    ↓
Data Layer (pure TypeScript)
    ↓
Downshift CLI
```

Each layer only knows about the one below it (Dependency Rule).

## Development

```bash
# Install dependencies
pnpm install

# Start dev server with mock data
pnpm dev

# Build for production
pnpm build
```

The dashboard starts in mock mode by default. Click the "Mock/Live" toggle
to switch to live data (requires a local server that proxies to the CLI).

## Components

| Component | Purpose |
|-----------|---------|
| `Header` | Session info, status, refresh controls |
| `MetricCard` | Single KPI with label, value, trend |
| `TokensDonut` | Work vs cache token savings chart |
| `DecisionTimeline` | Recent routing decisions list |
| `AgentHeatmap` | Savings breakdown by agent type |
| `CompactorStats` | Output compaction metrics |

## Data Flow

```
downshift session --panel --json
         ↓
    api.ts (fetch)
         ↓
    session.svelte.ts (state)
         ↓
    App.svelte (layout)
         ↓
    Components (render)
```

## Connecting to Live Data

The dashboard expects a proxy server at `/api/exec` that runs CLI commands.
For development without a proxy, mock data is used.

Future: Tauri/Electron integration for native CLI access.
