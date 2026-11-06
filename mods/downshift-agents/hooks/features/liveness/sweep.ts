// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0
//
// Keeps the counts honest when an end never arrives. One timer for the module
// marks silent running agents and in-progress tasks stale, redraws when
// something changed, and stops itself once nothing is running. Nothing is
// completed or deleted here; see agents/stale.ts and tasks/stale.ts.

import type { EngineInterface, Timer } from 'claude-code'

import { readAll, updateAgents, updateBoard } from '../../shared/state/atoms'
import { hasRunning, sweepStale } from '../agents/stale'
import { hasRunningTask, sweepTasks } from '../tasks/stale'

/** How often the sweep looks. */
export const SWEEP_EVERY_MS = 30_000

// One module load's timer: a reload cancels it with the old environment.
let sweeper: Timer | undefined

/** Starts the sweep if it is not running; call when an agent or task starts running. */
export function ensureSweeper($: EngineInterface): void {
  if (sweeper === undefined) {
    sweeper = $.clock.every(SWEEP_EVERY_MS, () => void sweepNow($))
  }
}

/** One sweep at the clock's now. */
export async function sweepNow($: EngineInterface): Promise<void> {
  const now = await $.clock.now()
  let isChanged = false
  await updateAgents($, list => {
    const next = sweepStale(list, now)
    isChanged = next.some((a, i) => a.status !== list[i]?.status)
    return next
  })
  await updateBoard($, board => {
    const next = sweepTasks(board, now)
    isChanged = isChanged || next.tasks.some((t, i) => t.status !== board.tasks[i]?.status)
    return next
  })
  if (isChanged) {
    $.ui.invalidate('ui.render')
  }
  const { agents, board } = await readAll($)
  if (!hasRunning(agents) && !hasRunningTask(board)) {
    sweeper?.cancel()
    sweeper = undefined
  }
}
