// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

import { test, expect } from '@playwright/test';
import { spawn, type ChildProcess } from 'node:child_process';
import { mkdtempSync, writeFileSync, rmSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { createHash } from 'node:crypto';
import { join } from 'node:path';

// E3: the same page-versus-API comparison as compare.spec.ts, against a temp
// event log. ?e2e=1 is not set, so the server cannot answer with the stub.

const PORT = 8811;
const BIN = process.env.DOWNSHIFT_BIN ?? join(process.cwd(), '..', 'downshift');

function sessionHash(id: string): string {
  return createHash('sha256').update(`harness-downshift-session-v1:${id}`).digest('hex');
}

function compact(n: number): string {
  if (n >= 1_000_000) return `${(n / 1_000_000).toFixed(1)}M`;
  if (n >= 1_000) return `${Math.round(n / 1_000)}k`;
  return String(n);
}

test('visible totals match a temp event log', async ({ browser, request }) => {
  const dir = mkdtempSync(join(tmpdir(), 'downshift-e2e-'));
  const hash = sessionHash('e2e-live');
  const ts = new Date().toISOString();
  const events = [
    {
      correlation_id: 'a1', timestamp: ts, session_id: hash, harness: 'cursor',
      outcome: 'rewrite_emitted', verdict: 'DOWNSHIFT',
      requested_model: 'grok-4.7-xhigh', final_model: 'composer-2.5',
    },
    {
      timestamp: ts, session_id: hash, harness: 'cursor', outcome: 'usage',
      linked_decision: 'a1', input_tokens: 1000,
    },
  ];
  writeFileSync(join(dir, 'events.jsonl'), events.map((row) => JSON.stringify(row)).join('\n') + '\n');
  writeFileSync(join(dir, 'exec-events.jsonl'), JSON.stringify({
    ts, family: 'go-test', raw_bytes: 4000, emitted_bytes: 0, compacted: true,
    session: hash, harness: 'cursor',
  }) + '\n');

  const child: ChildProcess = spawn(BIN, ['dashboard', `--port=${PORT}`, '--open=false', '-q'], {
    env: { ...process.env, DOWNSHIFT_STATE_DIR: dir, DOWNSHIFT_EVENT_LOG: join(dir, 'events.jsonl') },
    stdio: 'ignore',
  });
  const base = `http://127.0.0.1:${PORT}`;
  const page = await browser.newPage();
  try {
    await expect.poll(async () => (await request.get(`${base}/api/health`)).ok(), { timeout: 15_000 }).toBeTruthy();
    const sessions = await (await request.get(`${base}/api/sessions`)).json();
    const panel = await (await request.get(`${base}/api/session/current`)).json();
    const saved = (sessions.sessions as { tokens_saved: number }[]).reduce((sum, row) => sum + row.tokens_saved, 0);
    const compactorTokens = panel.compactor?.estimated_tokens_saved ?? 0;
    expect(saved).toBeGreaterThan(0);
    expect(compactorTokens).toBeGreaterThan(0);

    const expected = {
      footer: `${sessions.sessions.length} sessions · ${sessions.harnesses.length} harnesses`,
      tokens: compact(saved),
      compactor: compact(compactorTokens),
    };
    await page.goto(`${base}/`);
    await expect(page.getByTestId('dashboard-footer')).toHaveText(expected.footer);
    await page.getByTestId('tab-tokens').click();
    await expect(page.getByTestId('tokens-saved-total')).toHaveText(expected.tokens);
    await page.getByTestId('tab-compactor').click();
    await expect(page.getByTestId('compactor-tokens')).toHaveText(expected.compactor);
  } finally {
    await page.close();
    child.kill('SIGTERM');
    rmSync(dir, { recursive: true, force: true });
  }
});
