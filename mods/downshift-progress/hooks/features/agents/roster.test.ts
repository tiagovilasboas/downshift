// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

import { expect, test } from 'claude-code/testing'

import type { AgentRun, FinishedRun } from '../../../types'
import { FINISHED_SHOW_MS } from '../../shared/roster'
import { complete, spawn, world } from './roster-world'

const running = (w: ReturnType<typeof world>) => w.state<AgentRun[]>('running')
const finished = (w: ReturnType<typeof world>) => w.state<FinishedRun[]>('finished')

test('a spawn adds a running row keyed by tool_use id; its end moves it to finished', async ($, on) => {
  const w = world($, on)
  await spawn($, 'tu-1', 'Write tests')
  expect(running(w).map(r => [r.toolUse, r.description, r.model, r.agentId, r.startedAt])).toEqual([
    ['tu-1', 'Write tests', 'claude-sonnet-5-5', 'agent-1', 1_000_000],
  ])
  // sha256('harness-downshift-session-v1:tu-1'): the join key the binary prints.
  expect(running(w)[0]?.key).toMatch(/^[0-9a-f]{64}$/)

  await w.clock.advance(3_000)
  await complete($, 'agent-1')
  expect(running(w)).toEqual([])
  expect(finished(w).map(r => [r.toolUse, r.finishedAt, r.isFailed])).toEqual([['tu-1', 1_003_000, false]])
})

test('an error or an interrupted turn finishes the row as failed', async ($, on) => {
  const w = world($, on)
  await spawn($, 'tu-1', 'A')
  await spawn($, 'tu-2', 'B')
  await complete($, 'agent-1', 'error')
  await complete($, 'agent-2', 'aborted')
  expect(finished(w).map(r => r.isFailed)).toEqual([true, true])
})

test('the main loop turn and an unknown agent change nothing', async ($, on) => {
  const w = world($, on)
  await spawn($, 'tu-1', 'A')
  await $.turn.complete({ answer: '', durationMs: 5, isAborted: false, turnId: 'main', reason: 'answer' } as never)
  await complete($, 'ghost')
  expect(running(w).length).toBe(1)
  expect(finished(w)).toBeUndefined()
})

test('a finished row leaves the state exactly FINISHED_SHOW_MS after it ended, by the clock', async ($, on) => {
  const w = world($, on)
  expect(FINISHED_SHOW_MS).toBe(10_000)
  await spawn($, 'tu-1', 'A')
  await complete($, 'agent-1')
  await w.clock.advance(FINISHED_SHOW_MS - 1)
  expect(finished(w).length).toBe(1)
  await w.clock.advance(1)
  expect(finished(w)).toEqual([])
})

test('each row expires on its own time; a younger one stays', async ($, on) => {
  const w = world($, on)
  await spawn($, 'tu-1', 'old')
  await spawn($, 'tu-2', 'young')
  await complete($, 'agent-1')
  await w.clock.advance(6_000)
  await complete($, 'agent-2')
  await w.clock.advance(4_000)
  expect(finished(w).map(r => r.description)).toEqual(['young'])
  await w.clock.advance(6_000)
  expect(finished(w)).toEqual([])
})

test('turn steps add the agent tokens to its running row only', async ($, on) => {
  const w = world($, on)
  on('turn.step', async function* (_$, e) {
    return {
      turnId: e.turnId, index: e.index, answer: '', toolUses: [], stopReason: 'end_turn',
      usage: { input_tokens: 100, output_tokens: 20, cache_read_input_tokens: 5, cache_creation_input_tokens: 1, model: 'm' },
    } as never
  })
  await spawn($, 'tu-1', 'A')
  const step = { turnId: 't', index: 0, model: 'm', messageCount: 1 }
  const drain = async (input: object) => { for await (const _ of $.turn.step(input as never)) void _ }
  await drain({ ...step, agentId: 'agent-1' })
  await drain({ ...step, index: 1, agentId: 'agent-1' })
  await drain(step)
  expect(running(w)[0]?.tokens).toBe(252)
})
