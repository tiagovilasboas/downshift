// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0
//
// Observes the native task tools. A task moves only on what a tool call that
// succeeded states; the call itself is passed on untouched.

import type { EngineInterface, On } from 'claude-code'

import { glideTo } from '../../shared/glide/ticker'
import { updateBoard } from '../../shared/state/atoms'
import { ensureSweeper } from '../liveness/sweep'
import type { TaskBoard } from '../../../types'
import { progressOf, taskCreated, taskUpdated, todosWritten } from './board'
import type { TodoItem } from './board'
import { planEnded, planUpdated } from './plan'
import type { PlanInput } from './plan'
import { shareOf } from './shown'
import { stampTouched } from './stale'

const MAX_SUBJECT = 120
const PLAN_TOOL = 'mcp__ccd_turn__update_plan'
const WRAP_UP_TOOL = 'mcp__ccd_turn__wrap_up'

type Json = Record<string, unknown>
const isObject = (v: unknown): v is Json => typeof v === 'object' && v !== null

export function observeTasks(on: On): void {
  on('tool.call', { tool: 'TaskCreate' }, async ($, e, next) => {
    const ran = await next(e)
    const task = isObject(ran.result) && isObject(ran.result.task) ? ran.result.task : undefined
    if (ran.isError !== true && task !== undefined && typeof task.id === 'string') {
      const subject = (typeof task.subject === 'string' ? task.subject : e.subject).slice(0, MAX_SUBJECT)
      await moveBoard($, board => taskCreated(board, String(task.id), subject))
    }

    return ran
  })

  on('tool.call', { tool: 'TaskUpdate' }, async ($, e, next) => {
    const ran = await next(e)
    const ok = isObject(ran.result) && ran.result.success === true
    if (ran.isError !== true && ok) {
      await moveBoard($, board =>
        taskUpdated(board, e.taskId, {
          status: e.status,
          subject: e.subject?.slice(0, MAX_SUBJECT),
          addBlockedBy: e.addBlockedBy,
        }),
      )
    }

    return ran
  })

  on('tool.call', { tool: 'TodoWrite' }, async ($, e, next) => {
    const ran = await next(e)
    // A subagent's own todo list is not the session's progress.
    if (ran.isError !== true && ran.deny === undefined && e.agentId === undefined) {
      const todos: TodoItem[] = e.todos.map(t => ({ content: t.content.slice(0, MAX_SUBJECT), status: t.status }))
      await moveBoard($, board => todosWritten(board, todos))
    }

    return ran
  })

  on('tool.call', { tool: PLAN_TOOL }, async ($, e, next) => {
    const ran = await next(e)
    // A subagent's own plan is not the session's progress.
    const input = ran.isError !== true && ran.deny === undefined && e.agentId === undefined ? planInputOf(e) : undefined
    if (input !== undefined) {
      await moveBoard($, board => planUpdated(board, input))
    }

    return ran
  })

  on('tool.call', { tool: WRAP_UP_TOOL }, async ($, e, next) => {
    const ran = await next(e)
    if (ran.isError !== true && ran.deny === undefined && e.agentId === undefined) {
      await moveBoard($, planEnded)
    }

    return ran
  })
}

/** Applies a board change and lets the share done glide to its new value. */
async function moveBoard($: EngineInterface, fn: (board: TaskBoard) => TaskBoard): Promise<void> {
  const now = await $.clock.now()
  let before = 0
  let after = 0
  let isRunning = false
  await updateBoard($, board => {
    const next = stampTouched(board, fn(board), now)
    isRunning = next.tasks.some(t => t.status === 'running')
    before = shareOf(progressOf(board))
    after = shareOf(progressOf(next))
    return next
  })
  if (isRunning) {
    ensureSweeper($)
  }
  await glideTo($, 'tasks', before, after)
}

function planInputOf(v: unknown): PlanInput | undefined {
  if (!isObject(v)) {
    return undefined
  }
  if (Array.isArray(v.items)) {
    const items: TodoItem[] = []
    for (const it of v.items) {
      if (isObject(it) && typeof it.content === 'string') {
        const status = it.status === 'in_progress' || it.status === 'completed' ? it.status : 'pending'
        items.push({ content: it.content.slice(0, MAX_SUBJECT), status })
      }
    }
    return { items }
  }
  if (v.advance === true) {
    return { advance: true }
  }
  if (Array.isArray(v.done)) {
    const done = v.done.filter((n): n is number => typeof n === 'number')
    return { done, start: typeof v.start === 'number' ? v.start : undefined }
  }
  return undefined
}
