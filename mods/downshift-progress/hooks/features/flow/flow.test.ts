// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

import { expect, test } from 'claude-code/testing'

import { PROGRESS_TOOL, STEP_TOOL } from '.'

const text = (r: unknown): string => JSON.stringify(r)

test('the tools are declared at session start', async ($, on) => {
  const names: string[] = []
  on('command.register', (_: any, e: any) => ({ value: { command: e.name } }))
  on('tool.register', (_: any, e: any) => { names.push(e.name); return { value: { tool: `mcp__downshift-progress__${e.name}` } } })
  on('session.start', (_: any, e: any) => ({ cwd: e.cwd }))
  await $.session.start({ cwd: '/', surface: 'terminal', isInteractive: true })
  expect(names).toEqual(['progress', 'step'])
})

test('progress from the main session registers a flow; omitted fields keep their value', async ($) => {
  const a = await $.tool.call({ tool: PROGRESS_TOOL, title: 'Ship', total: 3, done: 1, phase: 'plan' } as never)
  expect(text(a)).toContain('Ship: 1/3')
  const b = await $.tool.call({ tool: PROGRESS_TOOL, done: 2 } as never)
  expect(text(b)).toContain('Ship: 2/3')
  const c = await $.tool.call({ tool: PROGRESS_TOOL, finished: true } as never)
  expect(text(c)).toContain('Ship: 3/3 (finished)')
})

test('progress from a subagent is ignored and changes nothing', async ($) => {
  await $.tool.call({ tool: PROGRESS_TOOL, title: 'Ship', total: 3, done: 1 } as never)
  const r = await $.tool.call({ tool: PROGRESS_TOOL, title: 'Hijack', total: 9, agentId: 'w1' } as never)
  expect(text(r)).toContain('ignored')
  const after = await $.tool.call({ tool: PROGRESS_TOOL, done: 1 } as never)
  expect(text(after)).toContain('Ship: 1/3')
})

test('bounds: done above total, total above 12, long title, too many tasks', async ($) => {
  await $.tool.call({ tool: PROGRESS_TOOL, title: 'B', total: 2 } as never)
  expect(text(await $.tool.call({ tool: PROGRESS_TOOL, done: 3 } as never))).toContain('cannot exceed total')
  expect(text(await $.tool.call({ tool: PROGRESS_TOOL, total: 13 } as never))).toContain('0 to 12')
  expect(text(await $.tool.call({ tool: PROGRESS_TOOL, title: 'x'.repeat(121) } as never))).toContain('title must be')
  const tasks = Array.from({ length: 13 }, (_, i) => ({ title: `t${i}` }))
  expect(text(await $.tool.call({ tool: PROGRESS_TOOL, tasks } as never))).toContain('at most 12')
  expect(text(await $.tool.call({ tool: PROGRESS_TOOL, phase: 'nope' } as never))).toContain('phase must be')
})

test('tasks with after[] are kept and set the total', async ($) => {
  const tasks = [{ title: 'A', tier: 'light' }, { title: 'B', tier: 'heavy', after: ['A'] }]
  expect(text(await $.tool.call({ tool: PROGRESS_TOOL, title: 'Dag', tasks } as never))).toContain('Dag: 0/2')
  const bad = { tool: PROGRESS_TOOL, tasks: [{ title: 'C', after: Array.from({ length: 21 }, () => 'A') }] }
  expect(text(await $.tool.call(bad as never))).toContain('at most 20')
  expect(text(await $.tool.call({ tool: PROGRESS_TOOL, tasks: [{ title: 'C', tier: 'giant' }] } as never))).toContain('tier must be')
})

test('a new title resets the flow; so does a call after finished', async ($) => {
  await $.tool.call({ tool: PROGRESS_TOOL, title: 'One', total: 4, done: 2, finished: true } as never)
  expect(text(await $.tool.call({ tool: PROGRESS_TOOL, title: 'One', total: 5 } as never))).toContain('One: 0/5')
  expect(text(await $.tool.call({ tool: PROGRESS_TOOL, title: 'Two' } as never))).toContain('Two: 0/0')
})

test('garbage input answers an error text and never throws', async ($) => {
  for (const bad of [{ title: 5 }, { total: 'x' }, { tasks: 'no' }, { tasks: [null] }, { finished: 'yes' }, { done: -1 }, { done: 1.5 }]) {
    const r = await $.tool.call({ tool: PROGRESS_TOOL, ...bad } as never)
    expect(text(r)).toContain('progress:')
  }
  for (const bad of [{ total: 'x' }, { done: 100 }, { note: 7 }, { note: 'n'.repeat(121) }]) {
    const r = await $.tool.call({ tool: STEP_TOOL, agentId: 'w1', ...bad } as never)
    expect(text(r)).toContain('step:')
  }
})

test('step from the main session is ignored', async ($) => {
  expect(text(await $.tool.call({ tool: STEP_TOOL, total: 3, done: 1 } as never))).toContain('ignored')
})

test('step is keyed by the calling agent and matches the planned task by description', async ($, on) => {
  on('agent.list', () => ({ value: [{ id: 'w1', description: 'B', type: 'general-purpose', status: 'running' }] }) as never)
  await $.tool.call({ tool: PROGRESS_TOOL, title: 'Dag', tasks: [{ title: 'A' }, { title: 'B' }] } as never)
  const r = await $.tool.call({ tool: STEP_TOOL, agentId: 'w1', total: 4, done: 1, note: 'reading' } as never)
  expect(text(r)).toContain('1/4')
  const again = await $.tool.call({ tool: STEP_TOOL, agentId: 'w1', done: 2 } as never)
  expect(text(again)).toContain('2/4')
  expect(text(await $.tool.call({ tool: STEP_TOOL, agentId: 'w1', done: 5 } as never))).toContain('cannot exceed')
  expect(text(again)).toContain('task B')
  // A new flow drops the old worker records: the stale total of 4 is gone.
  await $.tool.call({ tool: PROGRESS_TOOL, title: 'Other' } as never)
  expect(text(await $.tool.call({ tool: STEP_TOOL, agentId: 'w1', done: 1 } as never))).toContain('cannot exceed')
})

// The engine checks a registered tool's answer against its output shape: a
// string result, or a deny. An object result was refused live on 2026-10-24.
test('answers are a string result or a deny, never an object', async ($) => {
  const ok = (await $.tool.call({ tool: PROGRESS_TOOL, title: 'Shape', total: 2, done: 1 } as never)) as { result?: unknown }
  expect(typeof ok.result).toBe('string')
  const bad = (await $.tool.call({ tool: PROGRESS_TOOL, done: 9 } as never)) as { result?: unknown; deny?: unknown }
  expect(bad.result).toBeUndefined()
  expect(typeof bad.deny).toBe('string')
  const step = (await $.tool.call({ tool: STEP_TOOL, total: 2, done: 1, agentId: 'w1' } as never)) as { result?: unknown; deny?: unknown }
  expect(typeof (step.result ?? step.deny)).toBe('string')
})
