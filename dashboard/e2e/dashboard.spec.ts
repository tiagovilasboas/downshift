// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

import { test, expect, type ConsoleMessage } from '@playwright/test';

// B-W2 / DASH-2: behaviour sensor for the dashboard against #456.
// Every test loads the dashboard with ?e2e=1 so the server returns deterministic
// stub data. The suite proves the dashboard renders, tabs switch (URL changes),
// and the 3s auto-refresh runs without throwing in the console.

// E2E flag kept on every navigation, including tab switches via pushState.
const E2E = '/?e2e=1';

// collectErrors attaches console-error and pageerror listeners and returns the
// accumulated messages. Auto-refresh (#456) must not push anything here.
function collectErrors(page: import('@playwright/test').Page): string[] {
  const errors: string[] = [];
  page.on('console', (msg: ConsoleMessage) => {
    if (msg.type() === 'error') errors.push(msg.text());
  });
  page.on('pageerror', (err) => errors.push(err.message));
  return errors;
}

test('dashboard loads without error (200) and renders the shell', async ({ page }) => {
  const resp = await page.goto(E2E);
  expect(resp?.status()).toBe(200);

  // Header brand and the three tabs are present.
  await expect(page.getByText('downshift', { exact: true })).toBeVisible();
  await expect(page.getByTestId('tab-sessions')).toBeVisible();
  await expect(page.getByTestId('tab-tokens')).toBeVisible();
  await expect(page.getByTestId('tab-compactor')).toBeVisible();
});

test('sessions tab renders the stub sessions', async ({ page }) => {
  await page.goto(E2E);

  // The footer reports the counts derived from the stub (2 sessions, 2 harnesses).
  await expect(page.getByText('2 sessions · 2 harnesses')).toBeVisible();
});

test('switching tabs updates the URL and renders tab content', async ({ page }) => {
  await page.goto(E2E);

  // Tokens tab: aggregated tokens-saved label from the stub.
  await page.getByTestId('tab-tokens').click();
  await expect(page).toHaveURL(/[?&]tab=tokens/);
  await expect(page.getByText('tokens saved across all sessions')).toBeVisible();

  // Compactor tab: stub compactor block renders (token estimate text).
  await page.getByTestId('tab-compactor').click();
  await expect(page).toHaveURL(/[?&]tab=compactor/);

  // Back to sessions.
  await page.getByTestId('tab-sessions').click();
  await expect(page).toHaveURL(/[?&]tab=sessions/);
});

test('auto-refresh runs for a full cycle without console errors', async ({ page }) => {
  const errors = collectErrors(page);
  await page.goto(E2E);

  await expect(page.getByTestId('tab-sessions')).toBeVisible();

  // Auto-refresh is 3s; sessions list refresh is 10s. Wait past two panel
  // refreshes so a re-render during Motion animation would surface here.
  await page.waitForTimeout(7_000);

  // The shell is still interactive after the refreshes.
  await page.getByTestId('tab-tokens').click();
  await expect(page).toHaveURL(/[?&]tab=tokens/);

  expect(errors, `console errors during auto-refresh:\n${errors.join('\n')}`).toEqual([]);
});
