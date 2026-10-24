// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

import { expect, test } from 'claude-code/testing'

import type { AgentFigures, FlowTask } from '../../../types'
import { toolUseKey } from '../../shared/roster'
import { complete, spawn, world } from './roster-world'

const panelWith = async (toolUseId: string, extra: object = {}) =>
  JSON.stringify({
    session: 'abcd1234',
    spawns: [{ tool_use: await toolUseKey(toolUseId), state: 'switched', to_model: 'sonnet', observed_model: 'claude-haiku-5', saved_short: '58k' }],
    report: { tokens_saved_estimate: 1, scope: { kind: 'days', days: 30, label: 'last 30 days' }, display: { tokens_saved_short: '74k', scope_label: 'last 30 days' } },
    ...extra,
  })

test('figures are joined to the row by the hashed tool_use id', async ($, on) => {
  const w = world($, on)
  w.setPanel(await panelWith('tu-1'))
  await spawn($, 'tu-1', 'A')
  await spawn($, 'tu-2', 'B')
  await w.clock.settle()
  const figures = w.state<Record<string, AgentFigures>>('figures')
  expect(figures[await toolUseKey('tu-1')]).toEqual({ model: 'claude-haiku-5', state: 'switched', savedShort: '58k' })
  expect(figures[await toolUseKey('tu-2')]).toBeUndefined()
})

test('the first call asks for the report; later ones within 30 s are --no-report', async ($, on) => {
  const w = world($, on)
  w.setPanel(await panelWith('tu-1'))
  await spawn($, 'tu-1', 'A')
  await w.clock.settle()
  expect(w.argvs[0]).toEqual(['downshift', 'session', '--panel', '--session=S-1'])
  await spawn($, 'tu-2', 'B')
  await w.clock.settle()
  expect(w.argvs[1]).toContain('--no-report')
  await w.clock.advance(29_999)
  await w.clock.settle()
  expect(w.argvs.slice(1).every(a => a.includes('--no-report'))).toBe(true)
  await w.clock.advance(1)
  await w.clock.settle()
  expect(w.argvs.at(-1)).not.toContain('--no-report')
})

test('every 5 s while an agent runs, and no more once none does', async ($, on) => {
  const w = world($, on)
  w.setPanel(await panelWith('tu-1'))
  await spawn($, 'tu-1', 'A')
  await w.clock.settle()
  const afterSpawn = w.argvs.length
  await w.clock.advance(5_000)
  await w.clock.advance(5_000)
  expect(w.argvs.length).toBe(afterSpawn + 2)
  await complete($, 'agent-1')
  await w.clock.settle()
  const afterEnd = w.argvs.length
  await w.clock.advance(20_000)
  expect(w.argvs.length).toBe(afterEnd)
})

test('fail-open: no binary leaves the rows without figures', async ($, on) => {
  const w = world($, on)
  w.setPanel(null)
  await spawn($, 'tu-1', 'A')
  await w.clock.advance(5_000)
  await complete($, 'agent-1')
  expect(w.state('figures')).toBeUndefined()
  expect(w.state<unknown[]>('finished').length).toBe(1)
})

test('planned follows the flow and drops a task once an agent with its title starts', async ($, on) => {
  const w = world($, on)
  const tasks = [{ title: 'Build', tier: 'heavy' }, { title: 'Docs', tier: 'light', after: ['Build'] }]
  await $.tool.call({ tool: 'mcp__downshift-progress__progress', title: 'Ship', tasks } as never)
  expect(w.state<FlowTask[]>('planned').map(t => t.title)).toEqual(['Build', 'Docs'])
  await spawn($, 'tu-1', 'Build')
  expect(w.state<FlowTask[]>('planned').map(t => t.title)).toEqual(['Docs'])
  expect(w.state<{ tier: string }[]>('running')[0]?.tier).toBe('heavy')
  await w.clock.advance(60_000)
  await complete($, 'agent-1')
  await w.clock.advance(10_000)
  expect(w.state<FlowTask[]>('planned').map(t => t.title)).toEqual(['Docs'])
})
