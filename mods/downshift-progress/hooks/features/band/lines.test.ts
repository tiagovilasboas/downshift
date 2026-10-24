// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

import { expect, test } from 'claude-code/testing'

import type { BandPanel, BandReport, Flow } from '../../../types'
import { flowBar, flowLine, taskStatuses } from './flow-line'
import { runningSpawns } from './feed'
import type { Line } from './line'
import { summaryLine } from './summary-line'

const plain = (line: Line): string => line.map(s => s.text).join('')

const flow = (over: Partial<Flow> = {}): Flow => ({
  title: 'Ship',
  phase: 'delegate',
  total: 4,
  done: 1,
  finished: false,
  tasks: [{ title: 'A', tier: null, after: [] }, { title: 'B', tier: null, after: [] }, { title: 'C', tier: null, after: [] }, { title: 'D', tier: null, after: [] }],
  ...over,
})

test('flow line: title, phase, count, bar and one glyph per task', () => {
  const steps = { w1: { done: 1, total: 3, note: '', task: 'B' } }
  expect(plain(flowLine(flow(), steps, 'en', false))).toBe('Ship · delegate · 1/4 ██░░░░░░ · ✓●○○')
  expect(plain(flowLine(flow({ phase: null, tasks: [] }), {}, 'en', true))).toBe('Ship · 1/4 ##------')
})

test('flow line: pt-BR phase and ASCII glyphs', () => {
  expect(plain(flowLine(flow({ phase: 'review' }), {}, 'pt-BR', true))).toBe('Ship · revisar · 1/4 ##------ · +---')
})

test('a worker step that finished marks its task, and a finished flow marks all', () => {
  const steps = { w1: { done: 2, total: 2, note: '', task: 'C' } }
  expect(taskStatuses(flow({ done: 0 }), steps)).toEqual(['planned', 'planned', 'finished', 'planned'])
  expect(taskStatuses(flow({ finished: true, done: 4 }), {})).toEqual(['finished', 'finished', 'finished', 'finished'])
})

test('the bar follows the flow percent and never overflows', () => {
  expect(flowBar(flow({ done: 0 }), false)).toBe('░░░░░░░░')
  expect(flowBar(flow({ done: 4 }), false)).toBe('████████')
  expect(flowBar(flow({ total: 0, done: 0 }), true)).toBe('--------')
})

const panel = (over: Partial<BandPanel> = {}): BandPanel => ({
  summary: 'Downshift · 2 model · 1 held',
  modelSwitched: 2,
  effortChanged: 0,
  held: 1,
  spawns: [{ state: 'running' }, { state: 'switched' }, { state: 'running' }],
  ...over,
})

const report: BandReport = { tokensSaved: '73000 tokens', tokensSavedShort: '73k', scopeLabel: 'last 30 days' }

test('summary line prints the strings as given and the running count', () => {
  expect(plain(summaryLine(panel(), report, runningSpawns(panel()), 'en'))).toBe(
    'Downshift · 2 model · 1 held · Tokens saved · last 30 days: 73k · 2 running',
  )
  expect(plain(summaryLine(panel(), report, 1, 'pt-BR'))).toBe(
    'Downshift · 2 model · 1 held · Tokens economizados · last 30 days: 73k · 1 em execução',
  )
})

test('summary line tolerates a missing summary, short figure or scope', () => {
  const old = panel({ summary: null, effortChanged: 3 })
  expect(plain(summaryLine(old, null, 0, 'en'))).toBe('Downshift · 2 model · 3 effort · 1 held')
  expect(plain(summaryLine(old, { tokensSaved: '63000 tokens', tokensSavedShort: null, scopeLabel: null }, 0, 'en'))).toBe(
    'Downshift · 2 model · 3 effort · 1 held · Tokens saved: 63000 tokens',
  )
  expect(summaryLine(panel({ summary: null, modelSwitched: 0, held: 0 }), null, 0, 'en')).toEqual([])
  expect(summaryLine(null, report, 2, 'en').length).toBe(3)
})

test('running subagents are counted from the panel, none without one', () => {
  expect(runningSpawns(panel())).toBe(2)
  expect(runningSpawns(null)).toBe(0)
})
