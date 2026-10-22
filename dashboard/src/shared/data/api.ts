// Shared: API client for Downshift REST server
import type { PanelOut } from './types';

const DEFAULT_API_URL = 'http://127.0.0.1:8787';

function getApiUrl(): string {
  // @ts-expect-error Vite injects VITE_ prefixed env vars
  const customUrl = globalThis.VITE_DOWNSHIFT_API_URL;
  return customUrl || DEFAULT_API_URL;
}

export async function fetchPanel(sessionId?: string): Promise<PanelOut> {
  const baseUrl = getApiUrl();
  const endpoint = sessionId 
    ? `${baseUrl}/api/session/${sessionId}`
    : `${baseUrl}/api/session/current`;

  const res = await fetch(endpoint);
  
  if (!res.ok) {
    const error = await res.json().catch(() => ({ error: res.statusText }));
    throw new Error(error.error || `API error: ${res.status}`);
  }

  return res.json();
}

export async function checkHealth(): Promise<boolean> {
  try {
    const res = await fetch(`${getApiUrl()}/api/health`);
    return res.ok;
  } catch {
    return false;
  }
}

export function mockPanel(): PanelOut {
  return {
    session: 'a1b2c3d4',
    switched: 12,
    model_switched: 10,
    effort_changed: 2,
    held: 3,
    held_guardrail: 1,
    upshift_not_applied: 0,
    observed: 15,
    tokens: {
      children: 15,
      saved_children: 12,
      work_tokens: 450000,
      work_saved: 315000,
      cache_tokens: 1200000,
      cache_saved: 720000,
    },
    tokens_line: 'saved 70% of work · 60% of cache',
    by_agent_type: [
      {
        agent_type: 'general-purpose',
        tokens: { children: 10, saved_children: 8, work_tokens: 300000, work_saved: 210000, cache_tokens: 800000, cache_saved: 480000 },
        line: 'general-purpose: 70% work · 60% cache',
      },
    ],
    spawns: [
      {
        tool_use: 'abc123',
        state: 'switched',
        from_model: 'claude-opus-4',
        to_model: 'claude-sonnet-4',
        label: 'opus → sonnet',
        tokens: { children: 1, saved_children: 1, work_tokens: 45000, work_saved: 45000, cache_tokens: 120000, cache_saved: 120000 },
        tokens_label: '165k tokens · saved from opus',
      },
      {
        tool_use: 'def456',
        state: 'held',
        from_model: 'claude-sonnet-4',
        to_model: 'claude-sonnet-4',
        label: 'held: original model kept',
        tokens: { children: 1, saved_children: 0, work_tokens: 30000, work_saved: 0, cache_tokens: 80000, cache_saved: 0 },
        tokens_label: '110k tokens',
      },
      // Running agent (empty tokens_label)
      {
        tool_use: 'jkl012',
        state: 'switched',
        from_model: 'claude-opus-4',
        to_model: 'claude-sonnet-4',
        label: 'opus → sonnet',
        tokens: { children: 0, saved_children: 0, work_tokens: 0, work_saved: 0, cache_tokens: 0, cache_saved: 0 },
        tokens_label: '',
      },
    ],
    compactor: {
      scope: 'all',
      commands: 47,
      compacted: 42,
      raw_bytes: 1250000,
      saved_bytes: 890000,
      estimated_tokens_saved: 222500,
      token_estimate: 'saved_bytes / 4',
    },
    tokens_saved_estimate: 1257500,
  };
}
