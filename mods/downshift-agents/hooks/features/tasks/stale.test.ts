// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

import { expect, test } from 'claude-code/testing'

import type { TaskBoard } from '../../../types'
import { emptyBoard, progressOf, taskCreated, taskUpdated } from './board'
import { TASK_STALE_AFTER_MS, stampTouched, sweepTasks } from './stale'

const move = (board: TaskBoard, now: number, fn: (b: TaskBoard) => TaskBoard) => stampTouched(board, fn(board), now)

const started = () =>
  move(move(emptyBoard, 0, b => taskCreated(b, '1', 'a')), 100, b => taskUpdated(b, '1', { status: 'in_progress' }))

test('a task left in progress goes stale, is not completed and is not dropped', () => {
  const board = sweepTasks(started(), 100 + TASK_STALE_AFTER_MS + 1)
  expect(board.tasks.map(t => t.status)).toEqual(['stale'])
  expect(progressOf(board)).toMatchObject({ total: 1, completed: 0, running: 0, stale: 1 })
})

test('a task changed just inside the threshold stays running', () => {
  expect(sweepTasks(started(), 100 + TASK_STALE_AFTER_MS - 1).tasks.map(t => t.status)).toEqual(['running'])
})

test('a task stated in progress again runs again, and an untouched task keeps its stamp', () => {
  const stale = sweepTasks(started(), 100 + TASK_STALE_AFTER_MS + 1)
  const again = move(stale, 500_000, b => taskUpdated(b, '1', { status: 'in_progress' }))
  expect(again.tasks.map(t => [t.status, t.touchedAt])).toEqual([['running', 500_000]])
  expect(move(again, 600_000, b => b).tasks[0]?.touchedAt).toBe(500_000)
})
