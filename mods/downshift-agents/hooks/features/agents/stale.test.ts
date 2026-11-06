// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

import { expect, test } from 'claude-code/testing'

import type { Agent } from '../../../types'
import { spawnSettled, spawnStarted, toolUsed, turnCompleted } from './roster'
import { STALE_AFTER_MS, sweepStale } from './stale'

const spawn = (toolUseId: string, now = 0) => ({ toolUseId, label: toolUseId, type: 'general-purpose', now })
const done = (agentId: string, now: number) => ({ agentId, turnId: `u-${agentId}`, reason: 'answer' as const, now })
const statuses = (list: readonly Agent[]) => list.map(a => `${a.id}:${a.status}`)
const running = (list: readonly Agent[]) => list.filter(a => a.status === 'running').length

test('a spawn with no end goes stale after the threshold and stops counting as running', () => {
  const list = spawnSettled(spawnStarted([], spawn('t1')), 't1', 'a1', undefined)
  expect(running(sweepStale(list, STALE_AFTER_MS + 1))).toBe(0)
  expect(statuses(sweepStale(list, STALE_AFTER_MS + 1))).toEqual(['a1:stale'])
})

test('activity just inside the threshold keeps the agent running', () => {
  const list = toolUsed(spawnSettled(spawnStarted([], spawn('t1')), 't1', 'a1', undefined), 'a1', 'Read', 1000)
  expect(running(sweepStale(list, 1000 + STALE_AFTER_MS - 1))).toBe(1)
  expect(running(sweepStale(list, 1000 + STALE_AFTER_MS + 1))).toBe(0)
})

test('a late turn closes a stale agent, and a tool call revives it', () => {
  const stale = sweepStale(spawnSettled(spawnStarted([], spawn('t1')), 't1', 'a1', undefined), STALE_AFTER_MS + 1)
  expect(statuses(turnCompleted(stale, done('a1', 500_000)))).toEqual(['a1:completed'])
  expect(statuses(toolUsed(stale, 'a1', 'Read', 500_000))).toEqual(['a1:running'])
})

test('the sweep never deletes entries and leaves finished agents alone', () => {
  let list = spawnSettled(spawnStarted([], spawn('t1')), 't1', 'a1', undefined)
  list = turnCompleted(list, done('a1', 10))
  list = spawnStarted(list, spawn('t2'))
  expect(statuses(sweepStale(list, 10 * STALE_AFTER_MS))).toEqual(['a1:completed', 'tool:t2:stale'])
})

test('a turn under an unknown id closes the one open provisional entry', () => {
  let list = spawnSettled(spawnStarted([], spawn('t1')), 't1', 'a1', undefined)
  list = spawnStarted(list, spawn('t2'))
  list = turnCompleted(list, done('a1', 5))
  list = turnCompleted(list, done('ag2', 6))
  expect(statuses(list)).toEqual(['a1:completed', 'tool:t2:completed'])
})

test('with two open provisional entries an unknown id is not linked, and the sweep still retires it', () => {
  let list = spawnStarted(spawnStarted([], spawn('t1')), spawn('t2'))
  list = turnCompleted(list, done('ag9', 6))
  expect(list.map(a => a.id)).toEqual(['tool:t1', 'tool:t2', 'ag9'])
  expect(running(sweepStale(list, STALE_AFTER_MS + 10))).toBe(0)
})
