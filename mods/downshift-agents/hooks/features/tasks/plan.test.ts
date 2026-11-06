// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

import { expect, test } from 'claude-code/testing'

import { emptyBoard, progressOf, taskCreated, todosWritten } from './board'
import { planEnded, planUpdated } from './plan'

const items = [
  { content: 'a', status: 'completed' as const },
  { content: 'b', status: 'in_progress' as const },
  { content: 'c', status: 'pending' as const },
  { content: 'd', status: 'pending' as const },
]
const statuses = (b: ReturnType<typeof planUpdated>) => b.tasks.map(t => t.status)

test('items replace the whole plan, in_progress reads as running', () => {
  let b = planUpdated(emptyBoard, { items })
  expect(b.source).toBe('plan')
  expect(progressOf(b)).toMatchObject({ total: 4, completed: 1, running: 1, current: 'b', percent: 25 })
  b = planUpdated(b, { items: [{ content: 'x', status: 'pending' }] })
  expect(b.tasks.map(t => t.subject)).toEqual(['x'])
})

test('advance completes the running item and starts the next pending one', () => {
  const b = planUpdated(planUpdated(emptyBoard, { items }), { advance: true })
  expect(statuses(b)).toEqual(['completed', 'completed', 'running', 'pending'])
})

test('advance at the last item only completes it', () => {
  let b = planUpdated(emptyBoard, { items: [{ content: 'a', status: 'in_progress' }] })
  b = planUpdated(b, { advance: true })
  expect(statuses(b)).toEqual(['completed'])
  expect(progressOf(b).percent).toBe(100)
})

test('done and start move only the named items', () => {
  const b = planUpdated(planUpdated(emptyBoard, { items }), { done: [2], start: 3 })
  expect(statuses(b)).toEqual(['completed', 'completed', 'running', 'pending'])
})

test('out-of-range indices are ignored', () => {
  const base = planUpdated(emptyBoard, { items })
  const b = planUpdated(base, { done: [0, 9, -1, 1.5], start: 12 })
  expect(statuses(b)).toEqual(statuses(base))
})

test('advance or done without a plan change nothing', () => {
  expect(planUpdated(emptyBoard, { advance: true })).toBe(emptyBoard)
  expect(planUpdated(emptyBoard, { done: [1] })).toBe(emptyBoard)
})

test('native task tools outrank the plan, and the plan outranks TodoWrite', () => {
  const tasks = taskCreated(emptyBoard, '1', 'native')
  expect(planUpdated(tasks, { items })).toBe(tasks)
  const plan = planUpdated(emptyBoard, { items })
  expect(todosWritten(plan, [{ content: 'z', status: 'pending' }])).toBe(plan)
  const todos = todosWritten(emptyBoard, [{ content: 'z', status: 'pending' }])
  expect(planUpdated(todos, { items }).source).toBe('plan')
  expect(taskCreated(plan, '1', 'native').source).toBe('tasks')
})

test('wrap-up drops the plan, finished or not', () => {
  const open = planUpdated(planUpdated(emptyBoard, { items }), { advance: true })
  expect(planEnded(open)).toEqual(emptyBoard)
  expect(progressOf(planEnded(open)).total).toBe(0)
})

test('wrap-up keeps a board that is not the plan', () => {
  const todos = todosWritten(emptyBoard, [{ content: 't', status: 'pending' }])
  expect(planEnded(todos)).toBe(todos)
  const tasks = taskCreated(emptyBoard, '1', 'x')
  expect(planEnded(tasks)).toBe(tasks)
})
