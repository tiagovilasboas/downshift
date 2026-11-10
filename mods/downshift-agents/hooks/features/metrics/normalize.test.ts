// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

import { expect, test } from 'claude-code/testing'

import type { Agent, RoutingSpawn } from '../../../types'
import { bandSegments } from '../band/band'
import { treeRows } from '../dashboard/agents-section'
import { emptyBoard, progressOf } from '../tasks/board'
import { WARN } from '../../shared/ui/format'
import { uncutNote } from './cut'
import { agentCounts, modelClaim, reductionPercent } from './normalize'

const agent = (id: string, extra: Partial<Agent> = {}): Agent => ({
  id,
  label: id,
  type: 'general-purpose',
  status: 'running',
  startedAt: 0,
  runs: 1,
  tools: 0,
  seenTurns: [],
  ...extra,
})

const spawn = (extra: Partial<RoutingSpawn> = {}): RoutingSpawn => ({
  toolUseKey: 'k',
  state: 'rewritten',
  requested: 'claude-opus-5-5',
  final: 'claude-haiku-5-5',
  label: '',
  ...extra,
})

test('the model claim names its evidence, observed first', () => {
  expect(modelClaim(agent('a', { effectiveModel: 'm-obs', requestedModel: 'm-req' }), spawn())).toEqual({ model: 'm-obs', basis: 'observed' })
  expect(modelClaim(agent('a'), spawn({ observed: 'm-ds' }))).toEqual({ model: 'm-ds', basis: 'observed' })
  expect(modelClaim(agent('a', { requestedModel: 'm-req' }), spawn())).toEqual({ model: 'claude-haiku-5-5', basis: 'decided' })
  expect(modelClaim(agent('a', { requestedModel: 'm-req' }), undefined)).toEqual({ model: 'm-req', basis: 'requested' })
  expect(modelClaim(agent('a', { parentModel: 'p' }), undefined)).toEqual({ model: 'p', basis: 'inherited' })
  expect(modelClaim(agent('a'), undefined)).toEqual({ model: 'unknown', basis: 'inherited' })
})

test('reduction is saved over raw bytes, and absent with no raw bytes', () => {
  const c = { scope: 's', commands: 1, compacted: 1, rawBytes: 1000, savedBytes: 250, estimatedTokens: 62, method: 'm' }
  expect(reductionPercent(c)).toBe(25)
  expect(reductionPercent({ ...c, rawBytes: 0, savedBytes: 0 })).toBeUndefined()
})

test('orphan stubs are not counted as agents', () => {
  expect(agentCounts([agent('a'), agent('b', { isOrphan: true }), agent('c', { status: 'failed' })])).toEqual({ active: 1, stale: 0, total: 2, failed: 1 })
})

test('a child nests only under a parent that is listed', () => {
  const rows = treeRows([agent('a'), agent('b', { parentId: 'a' }), agent('c', { parentId: 'gone' })])
  expect(rows.map(r => `${r.agent.id}:${r.depth}`)).toEqual(['a:0', 'b:1', 'c:0'])
})

const input = {
  tasks: progressOf(emptyBoard),
  agents: { active: 1, stale: 0, total: 2, failed: 0 },
  compact: 18400,
  isDownshiftOk: true,
}

test('the band drops segments as the width narrows, the brand last', () => {
  const keys = (w: number) => bandSegments(input, w).map(s => s.key)
  expect(keys(120)).toEqual(['brand', 'tasks', 'agents', 'compact'])
  expect(keys(70)).toEqual(['brand', 'tasks', 'agents'])
  expect(keys(50)).toEqual(['brand', 'tasks'])
  expect(keys(20)).toEqual(['brand'])
})

test('the band draws an honest zero muted, never as a WARN figure', () => {
  type Extra = Partial<typeof input> & { runs?: { commands: number; compacted: number } }
  const cases: [string, Extra, string, boolean][] = [
    ['saving > 0', { compact: 14400, runs: { commands: 45, compacted: 3 } }, 'Compact ~14.4k*', true],
    ['runs but none cut', { compact: 0, runs: { commands: 45, compacted: 0 } }, 'Compact 0/45', false],
    ['no runs', { compact: 0, runs: { commands: 0, compacted: 0 } }, 'Compact —', false],
    ['no run counts', { compact: 0 }, 'Compact —', false],
    ['unavailable', { isDownshiftOk: false, compact: undefined }, 'Downshift n/a', false],
  ]
  for (const [name, extra, text, isWarn] of cases) {
    const seg = bandSegments({ ...input, ...extra }, 120).find(s => s.key === 'compact')
    expect(seg?.text, name).toBe(text)
    expect(seg?.parts.some(p => p.color === WARN), name).toBe(isWarn)
  }
})

test('the pane says plainly that nothing was cut, only when runs happened and none were cut', () => {
  const figures = (commands: number, compacted: number) => ({
    scope: 'this session',
    commands,
    compacted,
    rawBytes: 133643,
    savedBytes: compacted === 0 ? 0 : 5000,
    estimatedTokens: compacted === 0 ? 0 : 1250,
    method: '',
    families: [],
  })
  const cases: [string, ReturnType<typeof figures>, string | undefined][] = [
    ['saving > 0', figures(45, 3), undefined],
    ['runs, none cut', figures(45, 0), '45 runs, none cut: outputs were small, compound, or outside the allowlist'],
    ['no runs', figures(0, 0), undefined],
  ]
  for (const [name, c, want] of cases) {
    expect(uncutNote(c), name).toBe(want)
  }
})

test('the band shows no task count it does not have, and marks the token estimate', () => {
  const segs = bandSegments(input, 120)
  expect(segs.find(s => s.key === 'tasks')?.text).toBe('Tasks —')
  expect(segs.find(s => s.key === 'compact')?.text).toBe('Compact ~18.4k*')
  expect(bandSegments({ ...input, isDownshiftOk: false }, 120).find(s => s.key === 'compact')?.text).toBe('Downshift n/a')
})
