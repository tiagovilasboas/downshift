// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

import { expect, mock, test } from 'claude-code/testing'

import { PANEL_JSON } from './panel.fixture'

const UNCUT = { isStdoutTruncated: false, isStderrTruncated: false }
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

test('a read dropped by the gap leaves one trailing read, not a lost update', async ($, on) => {
  const clock = mock.clock(on, { now: 1_000_000 })
  let reads = 0
  on('session.id', () => ({ value: 's' }))
  on('process.run', () => {
    reads += 1
    return { value: { exitCode: 0, stdout: PANEL_JSON, stderr: '', ...UNCUT } }
  })
  on('agent.spawn', (_, e) => ({ model: 'claude-haiku-5-5', agentId: `agent-${e.tool_use_id}` }))

  await $.agent.spawn(spawnInput('t1'))
  await $.agent.spawn(spawnInput('t2'))
  await $.agent.spawn(spawnInput('t3'))
  expect(reads).toBe(1)
  await clock.advance(3000)
  expect(reads).toBe(2)
  await clock.advance(10_000)
  expect(reads).toBe(2)
})
