// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0
//
// /ds-agents report: prints `downshift report` (markdown by default, with the
// Compactor broken down per family) inside the session. The text is
// Downshift's own; a missing binary or a failed run says so instead.

import type { EngineInterface } from 'claude-code'

// The report reads the whole 30-day window, so it gets more time than the panel.
const REPORT_TIMEOUT_MS = 10000

export async function report($: EngineInterface): Promise<string> {
  try {
    const run = await $.process.run(['downshift', 'report'], { timeoutMs: REPORT_TIMEOUT_MS })
    if (run.exitCode !== 0) {
      return `Downshift report unavailable: downshift exited ${run.exitCode}.`
    }
    const text = run.stdout.trim()
    return text === '' ? 'Downshift report is empty: no events recorded yet.' : text
  } catch {
    return 'Downshift report unavailable: downshift is not on PATH or did not answer.'
  }
}
