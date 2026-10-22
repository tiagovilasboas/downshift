// Shared: Session state management with Svelte 5 runes
import { fetchPanel } from '../data/api';
import type { PanelOut } from '../data/types';
import { summaryMetrics } from '../data/transform';

// Reactive state
let panel = $state<PanelOut | null>(null);
let error = $state<string | null>(null);
let loading = $state(false);
let lastUpdate = $state<Date | null>(null);
let currentSessionId = $state<string | null>(null);

// Derived values
const metrics = $derived(panel ? summaryMetrics(panel) : null);
const isConnected = $derived(!error && panel !== null);

// Actions
async function refresh(sessionId?: string) {
  loading = true;
  error = null;
  const idToFetch = sessionId ?? currentSessionId ?? undefined;
  try {
    panel = await fetchPanel(idToFetch);
    lastUpdate = new Date();
  } catch (e) {
    error = e instanceof Error ? e.message : 'Failed to connect to API';
    panel = null;
  } finally {
    loading = false;
  }
}

function setSessionId(id: string | null) {
  currentSessionId = id;
  refresh();
}

// Auto-refresh
let intervalId: ReturnType<typeof setInterval> | null = null;

function startAutoRefresh(intervalMs = 3000) {
  stopAutoRefresh();
  refresh();
  intervalId = setInterval(() => refresh(), intervalMs);
}

function stopAutoRefresh() {
  if (intervalId) {
    clearInterval(intervalId);
    intervalId = null;
  }
}

export function getSessionState() {
  return {
    get panel() { return panel; },
    get error() { return error; },
    get loading() { return loading; },
    get lastUpdate() { return lastUpdate; },
    get metrics() { return metrics; },
    get isConnected() { return isConnected; },
    get currentSessionId() { return currentSessionId; },
    refresh,
    setSessionId,
    startAutoRefresh,
    stopAutoRefresh,
  };
}
