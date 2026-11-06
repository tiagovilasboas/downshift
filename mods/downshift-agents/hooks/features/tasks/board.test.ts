// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

import { expect, test } from 'claude-code/testing'

import { emptyBoard, progressOf, taskCreated, taskUpdated, todosWritten } from './board'

test('no tasks: no total and no percentage', () => {
  const p = progressOf(emptyBoard)
  expect(p.total).toBe(0)
  expect(p.percent).toBeUndefined()
  expect(p.current).toBeUndefined()
})

test('task tools drive progress, current task and blocked state', () => {
  let b = taskCreated(emptyBoard, '1', 'Read router')
  b = taskCreated(b, '2', 'Run tests')
  b = taskCreated(b, '3', 'Update hooks')
  b = taskUpdated(b, '1', { status: 'completed' })
  b = taskUpdated(b, '2', { status: 'in_progress' })
  b = taskUpdated(b, '3', { addBlockedBy: ['2'] })
  const p = progressOf(b)
  expect(p).toEqual({ total: 3, completed: 1, running: 1, stale: 0, blocked: 1, failed: 0, cancelled: 0, current: 'Run tests', percent: 33 })
})

test('a deleted task is cancelled and leaves the total', () => {
  let b = taskCreated(emptyBoard, '1', 'a')
  b = taskCreated(b, '2', 'b')
  b = taskUpdated(b, '2', { status: 'deleted' })
  b = taskUpdated(b, '1', { status: 'completed' })
  expect(progressOf(b).total).toBe(1)
  expect(progressOf(b).percent).toBe(100)
  expect(progressOf(b).cancelled).toBe(1)
})

test('an update for an unknown task changes nothing', () => {
  const b = taskCreated(emptyBoard, '1', 'a')
  expect(taskUpdated(b, '9', { status: 'completed' })).toEqual(b)
})

test('a task created twice is listed once', () => {
  const b = taskCreated(taskCreated(emptyBoard, '1', 'a'), '1', 'a')
  expect(b.tasks.length).toBe(1)
})

test('TodoWrite counts when no task tool is used, and yields to them', () => {
  let b = todosWritten(emptyBoard, [
    { content: 'one', status: 'completed' },
    { content: 'two', status: 'in_progress' },
  ])
  expect(b.source).toBe('todos')
  expect(progressOf(b).percent).toBe(50)
  b = taskCreated(b, '1', 'real')
  b = todosWritten(b, [{ content: 'ignored', status: 'pending' }])
  expect(b.source).toBe('tasks')
  expect(b.tasks.map(t => t.subject)).toEqual(['real'])
})
