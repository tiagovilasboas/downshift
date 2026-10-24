// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

import { test, expect } from '@playwright/test';
import { mkdirSync, writeFileSync } from 'node:fs';
import { dirname, join } from 'node:path';

// DASH-E2E: the page must show the same counts the API returned. The stub
// (?e2e=1) is the fixture; this spec is the comparison, and it writes the
// two sides to test-results/evidence.json so a failed run keeps the numbers.

const E2E = '/?e2e=1';

type SessionRow = { tokens_saved: number; switched: number; spawn_count: number; harness: string };
type SessionsBody = { sessions: SessionRow[]; harnesses: string[] };
type PanelBody = { compactor?: { estimated_tokens_saved: number } };

function compact(n: number): string {
  if (n >= 1_000_000) return `${(n / 1_000_000).toFixed(1)}M`;
  if (n >= 1_000) return `${Math.round(n / 1_000)}k`;
  return String(n);
}

test('visible totals match the sessions and panel APIs', async ({ page, request }, testInfo) => {
  const sessionsRes = await request.get('/api/sessions?e2e=1');
  const panelRes = await request.get('/api/session/current?e2e=1');
  expect(sessionsRes.ok()).toBeTruthy();
  expect(panelRes.ok()).toBeTruthy();
  const sessions = (await sessionsRes.json()) as SessionsBody;
  const panel = (await panelRes.json()) as PanelBody;

  const saved = sessions.sessions.reduce((sum, row) => sum + row.tokens_saved, 0);
  const expected = {
    footer: `${sessions.sessions.length} sessions · ${sessions.harnesses.length} harnesses`,
    tokens: compact(saved),
    compactor: compact(panel.compactor?.estimated_tokens_saved ?? 0),
  };

  await page.goto(E2E);
  const footer = (await page.getByTestId('dashboard-footer').innerText()).trim();
  await page.getByTestId('tab-tokens').click();
  const tokens = (await page.getByTestId('tokens-saved-total').innerText()).trim();
  await page.getByTestId('tab-compactor').click();
  const compactor = (await page.getByTestId('compactor-tokens').innerText()).trim();

  const evidence = {
    at: new Date().toISOString(),
    api: { sessions: sessions.sessions.length, harnesses: sessions.harnesses, tokens_saved: saved, compactor: panel.compactor?.estimated_tokens_saved ?? 0 },
    page: { footer, tokens, compactor },
    expected,
  };
  const out = join(testInfo.project.outputDir, 'evidence.json');
  mkdirSync(dirname(out), { recursive: true });
  writeFileSync(out, JSON.stringify(evidence, null, 2));

  expect(footer).toBe(expected.footer);
  expect(tokens).toBe(expected.tokens);
  expect(compactor).toBe(expected.compactor);
});
