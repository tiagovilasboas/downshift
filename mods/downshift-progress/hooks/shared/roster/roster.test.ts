// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

import { expect, test } from 'claude-code/testing'

import type { AgentRun, FinishedRun } from '../../../types'
import { NO_REPORT } from '../data'
import { FINISHED_SHOW_MS } from './constants'
import { buildRoster } from './roster'

const run = (toolUse: string, key: string): AgentRun => ({
  toolUse, key, agentId: `a-${toolUse}`, description: `task ${toolUse}`, type: 'general-purpose',
  tier: null, model: 'spawned-model', startedAt: 100, tokens: 7,
})
const done = (toolUse: string, key: string, finishedAt: number, isFailed = false): FinishedRun => ({ ...run(toolUse, key), finishedAt, isFailed })

const base = { running: [], finished: [], planned: [], figures: {}, cache: NO_REPORT }

test('figures join by key; a row without a match keeps the spawn model and no figures', () => {
  const r = buildRoster({
    ...base,
    running: [run('t1', 'k1'), run('t2', 'k2'), run('t3', '')],
    figures: { k1: { model: 'observed', state: 'switched', savedShort: '58k' } },
  }, 200)
  expect(r.running.map(x => [x.id, x.model, x.savedShort, x.state])).toEqual([
    ['t1', 'observed', '58k', 'switched'],
    ['t2', 'spawned-model', null, null],
    ['t3', 'spawned-model', null, null],
  ])
})

test('a finished row is shown at FINISHED_SHOW_MS - 1 and gone at FINISHED_SHOW_MS', () => {
  const snap = { ...base, finished: [done('t1', 'k1', 1_000), done('t2', 'k2', 1_500, true)] }
  const at = (now: number) => buildRoster(snap, now).finished.map(x => [x.id, x.status])
  expect(at(1_000 + FINISHED_SHOW_MS - 1)).toEqual([['t1', 'finished'], ['t2', 'failed']])
  expect(at(1_000 + FINISHED_SHOW_MS)).toEqual([['t2', 'failed']])
  expect(at(1_500 + FINISHED_SHOW_MS)).toEqual([])
})

test('the report and the planned list pass through untouched', () => {
  const report = { tokensSavedEstimate: 1, scope: { kind: null, days: null, label: null }, display: { tokensSaved: null, tokensSavedShort: '74k', scopeLabel: 'x', routing: null, compactor: null }, compactor: null }
  const r = buildRoster({ ...base, planned: [{ title: 'p', tier: null, after: [] }], cache: { report, at: 5 } }, 0)
  expect(r.report).toBe(report)
  expect(r.planned.map(p => p.title)).toEqual(['p'])
})
