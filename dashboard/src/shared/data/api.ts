// Shared: API client for Downshift REST server
import type { PanelOut, SessionsResponse } from './types';

function getApiUrl(): string {
  // When served by the Go dashboard server, the SPA and the API share an
  // origin, whatever port it listens on (1337 by default, but the E2E harness
  // and `serve --port` use others). Only the Vite dev server (5173) needs to
  // reach the Go API cross-origin on 1337.
  if (typeof window !== 'undefined') {
    const port = window.location.port;
    if (port !== '5173') {
      return window.location.origin;
    }
  }
  // Dev mode: Vite on 5173, Go API on 1337
  return 'http://127.0.0.1:1337';
}

// e2eSuffix forwards the ?e2e=1 harness flag from the page URL onto API
// requests so the server serves deterministic stub data (B-W2 / DASH-2).
// Returns "" in normal use, so real requests are untouched.
function e2eSuffix(): string {
  if (typeof window === 'undefined') return '';
  return new URLSearchParams(window.location.search).get('e2e') === '1'
    ? '?e2e=1'
    : '';
}

export async function fetchSessions(): Promise<SessionsResponse> {
  const res = await fetch(`${getApiUrl()}/api/sessions${e2eSuffix()}`);

  if (!res.ok) {
    const error = await res.json().catch(() => ({ error: res.statusText }));
    throw new Error(error.error || `API error: ${res.status}`);
  }

  return res.json();
}

export async function fetchPanel(sessionId?: string): Promise<PanelOut> {
  const baseUrl = getApiUrl();
  const endpoint = sessionId
    ? `${baseUrl}/api/session/${sessionId}`
    : `${baseUrl}/api/session/current`;

  const res = await fetch(`${endpoint}${e2eSuffix()}`);

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
