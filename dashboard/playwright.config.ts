// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

import { defineConfig, devices } from '@playwright/test';

// B-W2 / DASH-2: Playwright harness for the Downshift dashboard. It drives the
// real embedded SPA (served by `downshift dashboard`) with the ?e2e=1 flag, so
// the render is deterministic and never reads the user's events.jsonl. This is
// the behaviour sensor for #456 (auto-refresh re-render + Motion instability):
// it proves the dashboard renders, tabs switch, and auto-refresh logs no errors.
//
// The webServer runs the prebuilt binary at DOWNSHIFT_BIN (set by the e2e
// script after `go build` + `pnpm build`), so the test exercises the same
// embedded bundle that ships.
const PORT = Number(process.env.DASHBOARD_E2E_PORT ?? 8799);
const BIN = process.env.DOWNSHIFT_BIN ?? '../downshift';

export default defineConfig({
  testDir: './e2e',
  timeout: 30_000,
  expect: { timeout: 10_000 },
  fullyParallel: false,
  forbidOnly: !!process.env.CI,
  retries: 0,
  reporter: [['list']],
  use: {
    baseURL: `http://127.0.0.1:${PORT}`,
    trace: 'retain-on-failure',
  },
  projects: [
    {
      name: 'chromium',
      use: { ...devices['Desktop Chrome'] },
    },
  ],
  webServer: {
    command: `${BIN} dashboard --port=${PORT} --open=false -q`,
    url: `http://127.0.0.1:${PORT}/api/health`,
    reuseExistingServer: !process.env.CI,
    timeout: 30_000,
  },
});
