// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

import { expect, test } from 'claude-code/testing'

import type { Agent } from '../../../types'
import {
  MAX_AGENTS,
  provisionalId,
  spawnDenied,
  spawnSettled,
  spawnStarted,
  toolUsed,
  turnCompleted,
} from './roster'

const spawn = (toolUseId: string, extra: Partial<Parameters<typeof spawnStarted>[1]> = {}) => ({
  toolUseId,
  label: `task ${toolUseId}`,
  type: 'general-purpose',
  now: 1000,
  ...extra,
})

const usage = { input: 10, output: 5, cacheRead: 100, cacheWrite: 0 }

test('a single agent goes from spawn to completed with its observed model', () => {
  let list: Agent[] = spawnStarted([], spawn('t1', { requestedModel: 'haiku', parentModel: 'claude-opus-5-5' }))
  expect(list[0]?.id).toBe(provisionalId('t1'))
  list = spawnSettled(list, 't1', 'a1', 'claude-haiku-5-5')
  list = toolUsed(list, 'a1', 'Read', 1100)
  list = turnCompleted(list, { agentId: 'a1', turnId: 'u1', reason: 'answer', model: 'claude-haiku-5-5', usage, now: 4000 })
  const a = list[0]
  expect(a?.id).toBe('a1')
  expect(a?.status).toBe('completed')
  expect(a?.requestedModel).toBe('haiku')
  expect(a?.resolvedModel).toBe('claude-haiku-5-5')
  expect(a?.effectiveModel).toBe('claude-haiku-5-5')
  expect(a?.tools).toBe(1)
  expect(a?.tokens?.output).toBe(5)
})

test('no model is called effective before a turn reports one', () => {
  let list = spawnSettled(spawnStarted([], spawn('t1', { requestedModel: 'haiku' })), 't1', 'a1', 'claude-haiku-5-5')
  expect(list[0]?.effectiveModel).toBeUndefined()
  list = turnCompleted(list, { agentId: 'a1', turnId: 'u1', reason: 'error', now: 2000 })
  expect(list[0]?.effectiveModel).toBeUndefined()
  expect(list[0]?.status).toBe('failed')
})

test('concurrent agents keep apart', () => {
  let list = spawnStarted([], spawn('t1'))
  list = spawnStarted(list, spawn('t2'))
  list = spawnSettled(list, 't2', 'a2', undefined)
  list = spawnSettled(list, 't1', 'a1', undefined)
  list = turnCompleted(list, { agentId: 'a2', turnId: 'u2', reason: 'aborted', now: 2000 })
  expect(list.map(a => `${a.id}:${a.status}`)).toEqual(['a1:running', 'a2:cancelled'])
})

test('a repeated turn adds nothing', () => {
  let list = spawnSettled(spawnStarted([], spawn('t1')), 't1', 'a1', undefined)
  const done = { agentId: 'a1', turnId: 'u1', reason: 'answer' as const, usage, now: 2000 }
  list = turnCompleted(list, done)
  list = turnCompleted(list, done)
  expect(list[0]?.tokens?.input).toBe(10)
})

test('the same spawn heard twice is one agent', () => {
  let list = spawnStarted([], spawn('t1'))
  list = spawnStarted(list, spawn('t1'))
  expect(list.length).toBe(1)
  expect(list[0]?.runs).toBe(1)
})

test('events heard before the spawn settled join the agent', () => {
  let list = spawnStarted([], spawn('t1'))
  list = toolUsed(list, 'a1', 'Bash', 1100)
  list = turnCompleted(list, { agentId: 'a1', turnId: 'u1', reason: 'answer', model: 'claude-sonnet-5-5', usage, now: 3000 })
  expect(list.length).toBe(2)
  list = spawnSettled(list, 't1', 'a1', 'sonnet')
  expect(list.length).toBe(1)
  expect(list[0]?.label).toBe('task t1')
  expect(list[0]?.status).toBe('completed')
  expect(list[0]?.tools).toBe(1)
  expect(list[0]?.effectiveModel).toBe('claude-sonnet-5-5')
  expect(list[0]?.isOrphan).toBeUndefined()
})

test('a resumed agent runs again under its id', () => {
  let list = spawnSettled(spawnStarted([], spawn('t1')), 't1', 'a1', undefined)
  list = turnCompleted(list, { agentId: 'a1', turnId: 'u1', reason: 'answer', now: 2000 })
  list = spawnStarted(list, spawn('t9', { resumeId: 'a1' }))
  expect(list.length).toBe(1)
  expect(list[0]?.status).toBe('running')
  expect(list[0]?.runs).toBe(2)
})

test('a denied spawn leaves no agent', () => {
  expect(spawnDenied(spawnStarted([], spawn('t1')), 't1')).toEqual([])
})

test('nested agents keep the parent the spawn named', () => {
  let list = spawnSettled(spawnStarted([], spawn('t1')), 't1', 'a1', undefined)
  list = spawnStarted(list, spawn('t2', { parentId: 'a1' }))
  expect(list[1]?.parentId).toBe('a1')
  expect(list[0]?.parentId).toBeUndefined()
})

test('the roster stays bounded, dropping finished agents first', () => {
  let list: Agent[] = []
  for (let i = 0; i < MAX_AGENTS + 20; i++) {
    list = spawnSettled(spawnStarted(list, spawn(`t${i}`)), `t${i}`, `a${i}`, undefined)
    if (i === 0) {
      list = turnCompleted(list, { agentId: 'a0', turnId: 'u0', reason: 'answer', now: 2000 })
    }
  }
  expect(list.length).toBe(MAX_AGENTS)
  expect(list.some(a => a.id === 'a0')).toBe(false)
})
