// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0
//
// `downshift session --panel` for the band. The engine's scan accepts `$.process`
// and `$.env` only spelled at the call site, so the shared `fetchPanel` (which
// takes them as a value) cannot be called from a hook; this keeps its contract:
// same argv, same 2 s budget, never throws, every failure is a value.

import type { EngineInterface } from 'claude-code'

import { DEFAULT_BINARY, PANEL_TIMEOUT_MS, classifyFailure, parsePanel } from '../../shared/data'
import type { PanelResult } from '../../shared/data'

export async function callPanel(
  $: Pick<EngineInterface, 'process' | 'env'>,
  sessionId: string,
  withReport: boolean,
): Promise<PanelResult> {
  if (sessionId === '') return { ok: false, reason: 'no-session' }
  try {
    const bin = (await $.env.get('DOWNSHIFT_BIN'))?.trim() || DEFAULT_BINARY
    const argv = [bin, 'session', '--panel', `--session=${sessionId}`, ...(withReport ? [] : ['--no-report'])]
    const run = await $.process.run(argv, { timeoutMs: PANEL_TIMEOUT_MS })
    if (run.exitCode !== 0) return { ok: false, reason: 'unavailable-surface' }
    const panel = parsePanel(run.stdout)
    return panel ? { ok: true, panel } : { ok: false, reason: 'bad-json' }
  } catch (err) {
    return { ok: false, reason: classifyFailure(err) }
  }
}
