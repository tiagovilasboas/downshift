// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0
//
// Staleness for tasks left in progress. A task changes only on what a tool
// result states, so one whose update never arrived would read "running"
// forever. After the threshold it reads `stale`: never completed, never
// dropped, and running again when a tool states it so.

import type { Task, TaskBoard } from '../../../types'

/** A task in progress this long without a change is no longer counted as running. */
export const TASK_STALE_AFTER_MS = 120_000

/** Stamps the tasks that are new, or changed status or subject, between two boards. */
export function stampTouched(before: TaskBoard, after: TaskBoard, now: number): TaskBoard {
  const prior = new Map(before.tasks.map(t => [t.id, t]))
  const tasks = after.tasks.map(t => {
    const old = prior.get(t.id)
    const isChanged = old === undefined || old.status !== t.status || old.subject !== t.subject
    return { ...t, touchedAt: isChanged ? now : old?.touchedAt }
  })
  return { ...after, tasks }
}

/** The board with silent in-progress tasks marked stale. */
export function sweepTasks(board: TaskBoard, now: number, staleAfterMs = TASK_STALE_AFTER_MS): TaskBoard {
  const isSilent = (t: Task) => t.status === 'running' && t.touchedAt !== undefined && now - t.touchedAt > staleAfterMs
  return { ...board, tasks: board.tasks.map(t => (isSilent(t) ? { ...t, status: 'stale' as const } : t)) }
}

export const hasRunningTask = (board: TaskBoard): boolean => board.tasks.some(t => t.status === 'running')
