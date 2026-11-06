// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0
//
// The plan the Claude desktop app keeps for a session (mcp__ccd_turn__update_plan).
// Three input shapes: `items` replaces the plan, `advance` completes the item in
// progress and starts the next pending one, `done`/`start` move named 1-based items.
// A plan outranks TodoWrite and yields to the native task tools (see board.ts).

import type { Task, TaskBoard } from '../../../types'
import { MAX_TASKS, emptyBoard, statusOf } from './board'
import type { TodoItem } from './board'

export type PlanInput = {
  items?: readonly TodoItem[]
  advance?: boolean
  done?: readonly number[]
  start?: number
}

export function planUpdated(board: TaskBoard, input: PlanInput): TaskBoard {
  if (board.source === 'tasks') {
    return board
  }
  if (input.items !== undefined) {
    return planReplaced(input.items)
  }
  if (board.source !== 'plan') {
    return board
  }
  if (input.advance === true) {
    return planAdvanced(board)
  }
  return planMarked(board, input.done ?? [], input.start)
}

/**
 * The turn's wrap-up (mcp__ccd_turn__wrap_up) closes the app's plan, so the
 * board drops it instead of keeping a finished or abandoned list on screen.
 * A board from the task tools or TodoWrite is not the plan and stays.
 */
export function planEnded(board: TaskBoard): TaskBoard {
  return board.source === 'plan' ? emptyBoard : board
}

function planReplaced(items: readonly TodoItem[]): TaskBoard {
  const tasks: Task[] = items.slice(0, MAX_TASKS).map((t, i) => ({
    id: `plan:${i}`,
    subject: t.content,
    status: statusOf(t.status),
    blockedBy: [],
  }))
  return { source: tasks.length === 0 ? 'none' : 'plan', tasks }
}

function planAdvanced(board: TaskBoard): TaskBoard {
  const next = board.tasks.findIndex(t => t.status === 'pending')
  const tasks = board.tasks.map((t, i) => {
    if (t.status === 'running' || t.status === 'stale') {
      return { ...t, status: 'completed' as const }
    }
    return i === next ? { ...t, status: 'running' as const } : t
  })
  return { source: 'plan', tasks }
}

function planMarked(board: TaskBoard, done: readonly number[], start: number | undefined): TaskBoard {
  const finished = new Set(done.filter(Number.isInteger).map(n => n - 1))
  const tasks = board.tasks.map((t, i) => {
    if (finished.has(i)) {
      return { ...t, status: 'completed' as const }
    }
    return i === (start ?? 0) - 1 ? { ...t, status: 'running' as const } : t
  })
  return { source: 'plan', tasks }
}
