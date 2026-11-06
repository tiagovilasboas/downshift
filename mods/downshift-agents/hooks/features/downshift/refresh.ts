// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0
//
// Runs `downshift session --panel` for this session and keeps the snapshot.
// One read at a time, at most one every MIN_GAP_MS, never from a render, and
// a missing binary or a failed run marks Downshift unavailable instead of
// breaking the panel.

import type { EngineInterface, Timer } from 'claude-code'

import { glideTo } from '../../shared/glide/ticker'
import { readAll, setDownshift } from '../../shared/state/atoms'
import type { DownshiftSnapshot } from '../../../types'
import { reductionPercent } from '../metrics/normalize'
import { parsePanel } from './panel'

const MIN_GAP_MS = 3000
const TIMEOUT_MS = 4000

// One module load's read state: a reload starts it over.
let running: Promise<void> | undefined
let lastAt = 0
// The one trailing read waiting for the gap to pass, so a dropped call is not lost.
let trailing: Timer | undefined

/**
 * Reads Downshift unless a read ran within MIN_GAP_MS; force skips the gap.
 * A read already running is shared: a forced caller waits for its result.
 * A call the gap or a running read drops leaves one trailing read behind, so
 * the last update is shown without waiting for the next event.
 */
export async function refresh($: EngineInterface, force = false): Promise<void> {
  if (running !== undefined) {
    if (force) {
      return running
    }
    scheduleTrailing($, MIN_GAP_MS)
    return
  }
  const now = await $.clock.now()
  if (!force && now - lastAt < MIN_GAP_MS) {
    scheduleTrailing($, MIN_GAP_MS - (now - lastAt))
    return
  }
  lastAt = now
  running = readAndKeep($, now)
  try {
    await running
  } finally {
    running = undefined
  }
}

function scheduleTrailing($: EngineInterface, delayMs: number): void {
  if (trailing !== undefined) {
    return
  }
  trailing = $.clock.every(Math.max(1, delayMs), () => {
    trailing?.cancel()
    trailing = undefined
    void refresh($)
  })
}

async function readAndKeep($: EngineInterface, now: number): Promise<void> {
  const next = await readPanel($, now)
  const { downshift: before } = await readAll($)
  await setDownshift($, next)
  // a figure seen for the first time grows from zero
  const to = cutOf(next)
  if (to !== undefined) {
    await glideTo($, 'compactor', cutOf(before) ?? 0, to)
  }
}

/** The Compactor's percent cut, when Downshift reported one. */
function cutOf(s: DownshiftSnapshot): number | undefined {
  return s.status === 'ok' && s.compactor !== undefined ? reductionPercent(s.compactor) : undefined
}

async function readPanel($: EngineInterface, now: number) {
  try {
    const session = await $.session.id()
    const run = await $.process.run(['downshift', 'session', '--panel', `--session=${session}`], {
      timeoutMs: TIMEOUT_MS,
    })
    if (run.exitCode !== 0) {
      return { status: 'unavailable' as const, reason: `downshift exited ${run.exitCode}`, at: now }
    }
    return parsePanel(run.stdout, now)
  } catch {
    return { status: 'unavailable' as const, reason: 'downshift is not on PATH or did not answer', at: now }
  }
}
