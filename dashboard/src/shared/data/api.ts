// Shared: API client for Downshift REST server
import type { PanelOut, SessionsResponse } from './types';

function getApiUrl(): string {
  // When served by Go dashboard server, use same origin
  if (typeof window !== 'undefined') {
    const port = window.location.port;
    // Dashboard served by Go (port 1337) or any production deployment
    if (port === '1337' || port === '') {
      return window.location.origin;
    }
  }
  // Dev mode: Vite on 5173, Go API on 1337
  return 'http://127.0.0.1:1337';
}

export async function fetchSessions(): Promise<SessionsResponse> {
  const res = await fetch(`${getApiUrl()}/api/sessions`);
  
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
