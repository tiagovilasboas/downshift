// Shared: Session state management with Svelte 5 runes
import { mockPanel, fetchPanel } from '../data/api';
import type { PanelOut } from '../data/types';
import { summaryMetrics } from '../data/transform';

// Reactive state
let panel = $state<PanelOut | null>(null);
let error = $state<string | null>(null);
let loading = $state(false);
let lastUpdate = $state<Date | null>(null);
let useMock = $state(true);

// Derived values
const metrics = $derived(panel ? summaryMetrics(panel) : null);
const isConnected = $derived(!error && panel !== null);

// Actions
async function refresh(sessionId?: string) {
  loading = true;
  error = null;
  try {
    if (useMock) {
      await new Promise((r) => setTimeout(r, 150));
      panel = mockPanel();
      if (panel) {
        panel.switched += Math.floor(Math.random() * 2);
        panel.tokens.work_saved += Math.floor(Math.random() * 10000);
        panel.tokens_saved_estimate += Math.floor(Math.random() * 5000);
      }
    } else {
      panel = await fetchPanel(sessionId);
    }
    lastUpdate = new Date();
  } catch (e) {
    error = e instanceof Error ? e.message : 'Unknown error';
  } finally {
    loading = false;
  }
}

// Auto-refresh
let intervalId: ReturnType<typeof setInterval> | null = null;

function startAutoRefresh(intervalMs = 3000) {
  stopAutoRefresh();
  refresh();
  intervalId = setInterval(refresh, intervalMs);
}

function stopAutoRefresh() {
  if (intervalId) {
    clearInterval(intervalId);
    intervalId = null;
  }
}

function toggleMock() {
  useMock = !useMock;
  refresh();
}

export function getSessionState() {
  return {
    get panel() { return panel; },
    get error() { return error; },
    get loading() { return loading; },
    get lastUpdate() { return lastUpdate; },
    get metrics() { return metrics; },
    get isConnected() { return isConnected; },
    get useMock() { return useMock; },
    refresh,
    startAutoRefresh,
    stopAutoRefresh,
    toggleMock,
  };
}
