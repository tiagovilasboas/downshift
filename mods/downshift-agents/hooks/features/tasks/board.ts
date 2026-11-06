// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0
//
// Task progress from three sources. Precedence: the native task tools
// (TaskCreate, TaskUpdate) win once used; then the desktop app's plan
// (mcp__ccd_turn__update_plan, see plan.ts); then TodoWrite. A lower source
// never overwrites a higher one, and a plan replaces a TodoWrite list. Only
// what a tool result states moves a task: an agent stopping never completes
// one, and no count is guessed.

import type { Task, TaskBoard, TaskStatus } from '../../../types'

export const MAX_TASKS = 200

export const emptyBoard: TaskBoard = { source: 'none', tasks: [] }

export type TodoItem = { content: string; status: 'pending' | 'in_progress' | 'completed' }

export function taskCreated(board: TaskBoard, id: string, subject: string): TaskBoard {
  if (board.tasks.some(t => t.id === id) && board.source === 'tasks') {
    return board
  }
  const base = board.source === 'tasks' ? board.tasks : []
  const task: Task = { id, subject, status: 'pending', blockedBy: [] }
  return { source: 'tasks', tasks: [...base, task].slice(-MAX_TASKS) }
}

export type TaskChange = {
  status?: 'pending' | 'in_progress' | 'completed' | 'deleted'
  subject?: string
  addBlockedBy?: readonly string[]
}

export function taskUpdated(board: TaskBoard, id: string, change: TaskChange): TaskBoard {
  if (board.source !== 'tasks' || !board.tasks.some(t => t.id === id)) {
    return board
  }
  const tasks = board.tasks.map(t => {
    if (t.id !== id) {
      return t
    }
    const blockedBy = [...new Set([...t.blockedBy, ...(change.addBlockedBy ?? [])])]
    return {
      ...t,
      subject: change.subject ?? t.subject,
      status: change.status === undefined ? t.status : statusOf(change.status),
      blockedBy,
    }
  })
  return { source: 'tasks', tasks }
}

/** TodoWrite states the whole list each time; the task tools and the plan win when used. */
export function todosWritten(board: TaskBoard, todos: readonly TodoItem[]): TaskBoard {
  if (board.source === 'tasks' || board.source === 'plan') {
    return board
  }
  const tasks = todos.slice(0, MAX_TASKS).map((t, i) => ({
    id: `todo:${i}`,
    subject: t.content,
    status: statusOf(t.status),
    blockedBy: [],
  }))
  return { source: tasks.length === 0 ? 'none' : 'todos', tasks }
}

export function statusOf(s: NonNullable<TaskChange['status']>): TaskStatus {
  switch (s) {
    case 'in_progress':
      return 'running'
    case 'completed':
      return 'completed'
    case 'deleted':
      return 'cancelled'
    default:
      return 'pending'
  }
}

export type Progress = {
  total: number
  completed: number
  running: number
  stale: number
  blocked: number
  failed: number
  cancelled: number
  /** The task in progress, else undefined: never a guess. */
  current?: string
  /** 0..100 over the tasks that count (cancelled ones do not); undefined with no tasks. */
  percent?: number
}

/** The board's counts. A pending task whose blocker is not done reads as blocked. */
export function progressOf(board: TaskBoard): Progress {
  const done = new Set(board.tasks.filter(t => t.status === 'completed').map(t => t.id))
  const shown = board.tasks.map(t =>
    t.status === 'pending' && t.blockedBy.some(b => !done.has(b)) ? { ...t, status: 'blocked' as const } : t,
  )
  const count = (s: TaskStatus) => shown.filter(t => t.status === s).length
  const cancelled = count('cancelled')
  const total = shown.length - cancelled
  const completed = count('completed')
  return {
    total,
    completed,
    running: count('running'),
    stale: count('stale'),
    blocked: count('blocked'),
    failed: count('failed'),
    cancelled,
    current: shown.find(t => t.status === 'running')?.subject,
    percent: total === 0 ? undefined : Math.round((completed / total) * 100),
  }
}

/** Where the board's tasks come from, as the views name it. */
export function sourceLabel(source: TaskBoard['source']): string {
  switch (source) {
    case 'todos':
      return 'from TodoWrite'
    case 'plan':
      return 'from the session plan'
    default:
      return 'from task tools'
  }
}
