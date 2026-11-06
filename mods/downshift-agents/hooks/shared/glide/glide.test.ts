// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

import { expect, test } from 'claude-code/testing'

import { shownProgress } from '../../features/tasks/shown'
import { GLIDE_MS, easeOut, isMoving, moving, shownValue } from './glide'

const half = { total: 4, completed: 2, running: 1, blocked: 0, failed: 0, cancelled: 0, percent: 50 }

test('the ease starts at 0, ends at 1, never runs back and clamps outside [0, 1]', () => {
  const steps = [0, 0.1, 0.25, 0.5, 0.75, 0.9, 1].map(easeOut)
  expect(steps[0]).toBe(0)
  expect(steps.at(-1)).toBe(1)
  expect(steps.every((v, i) => i === 0 || v > steps[i - 1])).toBe(true)
  expect([easeOut(-1), easeOut(2)]).toEqual([0, 1])
  // fast at first: more than half the way by a third of the time
  expect(easeOut(1 / 3)).toBeGreaterThan(0.5)
})

test('a shown value eases from where it stood to the live one, then holds the live one', () => {
  const g = { from: 20, at: 1000 }
  expect(shownValue(g, 80, 1000)).toBe(20)
  const mid = shownValue(g, 80, 1000 + GLIDE_MS / 2)
  expect(mid).toBeGreaterThan(20)
  expect(mid).toBeLessThan(80)
  expect(shownValue(g, 80, 1000 + GLIDE_MS)).toBe(80)
  expect(shownValue(undefined, 80, 1000)).toBe(80)
  // a figure that shrinks glides down the same way
  expect(shownValue({ from: 80, at: 0 }, 20, GLIDE_MS / 2)).toBeLessThan(80)
})

test('a glide moves for GLIDE_MS, and only the moving ones are kept', () => {
  const glides = { tasks: { from: 0, at: 0 }, compactor: { from: 0, at: 500 } }
  expect(isMoving(glides.tasks, GLIDE_MS - 1)).toBe(true)
  expect(isMoving(glides.tasks, GLIDE_MS)).toBe(false)
  expect(Object.keys(moving(glides, GLIDE_MS))).toEqual(['compactor'])
  expect(moving(glides, GLIDE_MS + 500)).toEqual({})
})

test('the task count and percent follow the bar while it glides, the other counts stay live', () => {
  const start = shownProgress(half, { from: 0, at: 0 }, 0)
  expect([start.share, start.completed, start.percent]).toEqual([0, 0, 0])
  expect(start.running).toBe(1)
  const mid = shownProgress(half, { from: 0, at: 0 }, GLIDE_MS / 2)
  expect(mid.percent).toBeGreaterThan(0)
  expect(mid.percent).toBeLessThan(50)
  expect(mid.completed).toBe(Math.round(mid.share * 4))
  expect(shownProgress(half, undefined, 0)).toEqual({ ...half, share: 0.5 })
  const none = shownProgress({ ...half, total: 0, completed: 0, percent: undefined }, undefined, 0)
  expect([none.share, none.percent]).toEqual([0, undefined])
})
