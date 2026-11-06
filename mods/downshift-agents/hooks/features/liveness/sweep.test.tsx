// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

import { expect, mock, test } from 'claude-code/testing'

import { PANEL_JSON } from '../downshift/panel.fixture'
import { STALE_AFTER_MS } from '../agents/stale'
import { SWEEP_EVERY_MS } from './sweep'

const UNCUT = { isStdoutTruncated: false, isStderrTruncated: false }
const SUMMARY = {
  command: 'ds-agents',
  args: 'summary',
  origin: { kind: 'composer' },
  presentation: { isFullscreen: false, columns: 100 },
} as const
const ORIGIN = { plugin: 'engine', tier: 'core' } as const

const spawnInput = (id: string) => ({
  tool_use_id: id,
  prompt: 'not stored',
  description: `job ${id}`,
  subagentType: 'general-purpose',
  provider: ORIGIN,
  parentModel: 'claude-opus-5-5',
  background: false,
  fork: false,
})

test('the tick marks a spawn with no end stale, and the summary says so', async ($, on) => {
  const clock = mock.clock(on, { now: 1000 })
  on('session.id', () => ({ value: 's' }))
  on('process.run', () => ({ value: { exitCode: 0, stdout: PANEL_JSON, stderr: '', ...UNCUT } }))
  on('command.run', () => ({ text: '' }))
  on('agent.spawn', (_, e) => ({ model: 'claude-haiku-5-5', agentId: `agent-${e.tool_use_id}` }))

  await $.agent.spawn(spawnInput('t1'))
  expect((await $.command.run(SUMMARY)).text).toContain('Agents: 1 active, 1 total, 0 failed')
  await clock.advance(STALE_AFTER_MS + 2 * SWEEP_EVERY_MS)
  const out = (await $.command.run(SUMMARY)).text
  expect(out).toContain('Agents: 0 active, 1 total, 0 failed, 1 stale')
})

test('a task left in progress goes stale on the tick', async ($, on) => {
  const clock = mock.clock(on, { now: 1000 })
  on('session.id', () => ({ value: 's' }))
  on('process.run', () => ({ value: { exitCode: 0, stdout: PANEL_JSON, stderr: '', ...UNCUT } }))
  on('command.run', () => ({ text: '' }))
  on('tool.call', (_, e) =>
    e.tool === 'TaskCreate' ? { result: { task: { id: '1', subject: 'a' } } } : { result: { success: true } },
  )

  await $.tool.call({ tool: 'TaskCreate', subject: 'a', description: 'x' })
  await $.tool.call({ tool: 'TaskUpdate', taskId: '1', status: 'in_progress' })
  expect((await $.command.run(SUMMARY)).text).toContain('1 running')
  await clock.advance(STALE_AFTER_MS + 2 * SWEEP_EVERY_MS)
  expect((await $.command.run(SUMMARY)).text).toContain('0 running, 0 blocked, 1 stale')
})
