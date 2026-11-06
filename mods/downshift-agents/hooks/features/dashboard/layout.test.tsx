// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0
//
// Layout tests: the pane's cards and the band's frame follow the room they
// get, and every card keeps its figures at any width. No live session runs here.

import type { On } from 'claude-code'
import { expect, mock, test } from 'claude-code/testing'
import type { Engine, MockClock } from 'claude-code/testing'

import { PANEL_JSON } from '../downshift/panel.fixture'
import { GLIDE_MS } from '../../shared/glide/glide'

const UNCUT = { isStdoutTruncated: false, isStderrTruncated: false }

const SUMMARY = {
  command: 'ds-agents',
  args: 'summary',
  origin: { kind: 'composer' },
  presentation: { isFullscreen: false, columns: 100 },
} as const

const pane = (surface: 'terminal' | 'desktop', bodyColumns: number) => ({
  plugin: 'downshift-agents',
  component: 'Pane',
  requestId: 'downshift-agents',
  surface,
  props: { title: 'Downshift Agents', isFocused: false, bodyColumns, placement: 'dock', scroll: { offset: 0, bodyRows: 40 }, view: {} },
} as const)

const band = (bodyColumns: number, maxRows: number) => ({
  plugin: 'downshift-agents',
  component: 'AbovePrompt',
  surface: 'terminal',
  props: { hasSurvey: false, isWorking: true, maxRows, bodyColumns, scroll: { offset: 0, bodyRows: maxRows }, view: {} },
} as const)

/** Downshift answers with the panel fixture; the summary command makes the first read. */
function withDownshift(on: On): MockClock {
  const clock = mock.clock(on, { now: 1000 })
  on('session.id', () => ({ value: 'layout' }))
  on('process.run', () => ({ value: { exitCode: 0, stdout: PANEL_JSON, stderr: '', ...UNCUT } }))
  on('command.run', () => ({ text: '' }))
  return clock
}

const framedBoxes = async (ui: { findAll: (q: { type: string }) => Promise<{ props: Record<string, unknown> }[]> }) =>
  (await ui.findAll({ type: 'Box' })).filter(b => b.props.borderStyle === 'round')

test('a desktop pane draws the tiles and panels with Svg and says Downshift is live', async ($, on) => {
  withDownshift(on)
  await $.command.run(SUMMARY)

  const ui = await $.ui.mount(pane('desktop', 60))
  expect(await ui.find({ type: 'Text', text: /DOWNSHIFT AGENTS/ })).toBeDefined()
  expect(await ui.find({ type: 'Text', text: /TASK PROGRESS no tasks reported yet/ })).toBeDefined()
  expect(await ui.find({ type: 'Text', text: /TOKENS SAVED/ })).toBeDefined()
  expect(await ui.find({ type: 'Text', text: /by the Compactor/ })).toBeDefined()
  expect(await ui.find({ type: 'Text', text: /ROUTING 40% of subagent tokens ran on a lighter model/ })).toBeDefined()
  expect(await ui.find({ type: 'Text', text: /^~14\.4k\*$/ })).toBeDefined()
  expect(await framedBoxes(ui)).toHaveLength(8)
  // collapsed, each panel keeps its headline figures and leaves the counts to Details
  expect(await ui.findAll({ type: 'Text', text: /^(Model changed|Raw \(measured\))$/ })).toHaveLength(0)
  const art = await ui.findAll({ type: 'Svg' })
  expect(art.some(s => s.props.alt === 'live')).toBe(true)
  expect(art.every(s => typeof s.props.source === 'string' && s.props.source.startsWith('<svg'))).toBe(true)
  await ui.unmount()
})

test('a narrow desktop pane stacks the tiles and keeps every figure', async ($, on) => {
  withDownshift(on)
  await $.command.run(SUMMARY)

  const ui = await $.ui.mount(pane('desktop', 40))
  const stacks = (await ui.findAll({ type: 'Box' })).filter(b => b.props.flexDirection === 'column' && b.props.gap === 'sm')
  expect(stacks.length).toBeGreaterThan(0)
  for (const label of [/AGENTS/, /ON A CHEAPER MODEL/, /^Tokens saved$/, /^Output cut$/]) {
    expect(await ui.find({ type: 'Text', text: label })).toBeDefined()
  }
  await ui.unmount()
})

test('a narrow pane drops the frames and keeps every card', async ($, on) => {
  withDownshift(on)
  await $.command.run(SUMMARY)

  const ui = await $.ui.mount(pane('terminal', 24))
  expect(await framedBoxes(ui)).toHaveLength(0)
  expect(await ui.findAll({ type: 'Svg' })).toHaveLength(0)
  for (const title of [/TASK PROGRESS/, /SUBAGENTS/, /COMPACTOR/, /ROUTING/, /^Tokens saved$/]) {
    expect(await ui.find({ type: 'Text', text: title })).toBeDefined()
  }
  await ui.unmount()
})

test('the band never takes a frame, so every column goes to its figures', async ($, on) => {
  withDownshift(on)
  on('ui.render', ($, e) => {
    const { Text } = $.ui.resolve(e)
    return <Text>engine</Text>
  })
  await $.command.run(SUMMARY)

  let ui = await $.ui.mount(band(120, 10))
  expect(await framedBoxes(ui)).toHaveLength(0)
  expect(await ui.find({ type: 'Text', text: /Compact ~14\.4k\*/ })).toBeDefined()
  await ui.unmount()

  ui = await $.ui.mount(band(120, 3))
  expect(await framedBoxes(ui)).toHaveLength(0)
  expect(await ui.find({ type: 'Text', text: /Downshift Agents/ })).toBeDefined()
  await ui.unmount()

  ui = await $.ui.mount(band(50, 10))
  expect(await framedBoxes(ui)).toHaveLength(0)
  await ui.unmount()
})

const ORIGIN = { plugin: 'engine', tier: 'core' } as const

test('terminal agent rows are single lines with no vertical padding and one dim rule between them', async ($, on) => {
  withDownshift(on)
  on('agent.spawn', (_, e) => ({ model: 'claude-haiku-5-5', agentId: `agent-${e.tool_use_id}` }))
  await $.command.run(SUMMARY)
  for (const id of ['a1', 'a2']) {
    await $.agent.spawn({ tool_use_id: id, prompt: 'x', description: `agent ${id}`, subagentType: 'general-purpose', provider: ORIGIN, parentModel: 'claude-opus-5-5', background: false, fork: false })
  }

  const ui = await $.ui.mount(pane('terminal', 80))
  expect(await ui.find({ type: 'Text', text: /agent a1/ })).toBeDefined()
  expect(await ui.findAll({ type: 'Text', text: /^─+$/ })).toHaveLength(1)
  const boxes = await ui.findAll({ type: 'Box' })
  expect(boxes.filter(b => b.props.paddingY !== undefined || b.props.paddingTop !== undefined || b.props.marginY !== undefined)).toHaveLength(0)
  await ui.unmount()
})

test('desktop agent rows are not cards: no per-agent frame, no vertical padding, one rule between them', async ($, on) => {
  withDownshift(on)
  on('agent.spawn', (_, e) => ({ model: 'claude-haiku-5-5', agentId: `agent-${e.tool_use_id}` }))
  await $.command.run(SUMMARY)
  for (const id of ['a1', 'a2']) {
    await $.agent.spawn({ tool_use_id: id, prompt: 'x', description: `agent ${id}`, subagentType: 'general-purpose', provider: ORIGIN, parentModel: 'claude-opus-5-5', background: false, fork: false })
  }

  const ui = await $.ui.mount(pane('desktop', 80))
  expect(await ui.find({ type: 'Text', text: /agent a1/ })).toBeDefined()
  expect((await ui.findAll({ type: 'Svg' })).filter(s => s.props.alt === 'divider')).toHaveLength(1)
  expect(await framedBoxes(ui)).toHaveLength(8)
  const boxes = await ui.findAll({ type: 'Box' })
  // only the panel frames keep a title margin; no body text line is padded or gapped
  expect(boxes.filter(b => b.props.paddingY !== undefined && b.props.borderStyle !== 'round')).toHaveLength(0)
  expect(boxes.filter(b => b.props.flexDirection === 'column' && b.props.gap === 'xs')).toHaveLength(0)
  await ui.unmount()
})

const spawnTwo = async ($: Engine) => {
  for (const id of ['a1', 'a2']) {
    await $.agent.spawn({ tool_use_id: id, prompt: 'x', description: `task ${id}`, subagentType: 'general-purpose', provider: ORIGIN, parentModel: 'claude-opus-5-5', background: false, fork: false })
  }
}

test('the orchestrator is the root row above its agents, once, with its model, and the agents show their task', async ($, on) => {
  withDownshift(on)
  on('agent.spawn', (_, e) => ({ model: 'claude-haiku-5-5', agentId: `agent-${e.tool_use_id}` }))
  await $.command.run(SUMMARY)
  await spawnTwo($)

  for (const columns of [80, 24]) {
    const ui = await $.ui.mount(pane('terminal', columns))
    expect(await ui.findAll({ type: 'Text', text: /^◆ orchestrator/ })).toHaveLength(1)
    expect(await ui.find({ type: 'Text', text: /opus-5-5/ })).toBeDefined()
    // the root row is not an agent: the counts stay the agents'
    expect(await ui.find({ type: 'Text', text: /SUBAGENTS 2 active · 2 total/ })).toBeDefined()
    expect(await ui.find({ type: 'Text', text: /· task a1/ })).toBeDefined()
    // nothing is pinned: no absolute position anywhere in the pane
    expect((await ui.findAll({ type: 'Box' })).filter(b => b.props.position !== undefined)).toHaveLength(0)
    await ui.unmount()
  }
})

test('with no subagent there is no orchestrator row to duplicate', async ($, on) => {
  withDownshift(on)
  await $.command.run(SUMMARY)

  const ui = await $.ui.mount(pane('terminal', 80))
  expect(await ui.findAll({ type: 'Text', text: /orchestrator/ })).toHaveLength(0)
  expect(await ui.find({ type: 'Text', text: /None yet in this session/ })).toBeDefined()
  await ui.unmount()
})

test('desktop: the orchestrator is the root row and a 2px gap sits under the SUBAGENTS title', async ($, on) => {
  withDownshift(on)
  on('agent.spawn', (_, e) => ({ model: 'claude-haiku-5-5', agentId: `agent-${e.tool_use_id}` }))
  await $.command.run(SUMMARY)
  await spawnTwo($)

  const ui = await $.ui.mount(pane('desktop', 80))
  expect(await ui.findAll({ type: 'Text', text: /^orchestrator$/ })).toHaveLength(1)
  expect(await ui.find({ type: 'Text', text: /^task a1$/ })).toBeDefined()
  // one truncating line for the task, the model claim and type on the next; the basis is a mark
  expect(await ui.find({ type: 'Text', text: /opus-5-5 ↑ · general-purpose/ })).toBeDefined()
  expect(await ui.findAll({ type: 'Text', text: /\(decided\)/ })).toHaveLength(0)
  const gaps = (await ui.findAll({ type: 'Svg' })).filter(s => s.props.alt === 'title gap')
  // one under every section title with content
  expect(gaps).toHaveLength(4)
  expect(gaps.every(g => g.props.height === 6)).toBe(true)
  await ui.unmount()
})

test('the pane shows tokens below the session model per harness, never an unobserved row', async ($, on) => {
  withDownshift(on)
  await $.command.run(SUMMARY)

  for (const surface of ['terminal', 'desktop'] as const) {
    const ui = await $.ui.mount(pane(surface, 100))
    expect(await ui.find({ type: 'Text', text: /^On a lighter model$/ })).toBeDefined()
    expect(await ui.find({ type: 'Text', text: /12\.0k tokens · 1 of 2 agents/ })).toBeDefined()
    expect(await ui.find({ type: 'Text', text: /^On the session model$/ })).toBeDefined()
    expect(await ui.find({ type: 'Text', text: /18\.0k tokens/ })).toBeDefined()
    expect(await ui.find({ type: 'Text', text: /codex · no session model/ })).toBeDefined()
    expect(await ui.find({ type: 'Text', text: /^(?!Premium preserved|unobserved \(no spawn).*unobserved/ })).toBeUndefined()
    await ui.unmount()
  }
})

test('the pane draws the direct-calls line and the rewrite figure as card rows', async ($, on) => {
  withDownshift(on)
  await $.command.run(SUMMARY)

  for (const surface of ['terminal', 'desktop'] as const) {
    const ui = await $.ui.mount(pane(surface, 100))
    expect(await ui.find({ type: 'Text', text: /^Direct calls$/ })).toBeDefined()
    expect(await ui.find({ type: 'Text', text: /3 · ~1\.2k\* tokens/ })).toBeDefined()
    expect(await ui.find({ type: 'Text', text: /direct calls, no session: 3/ })).toBeDefined()
    expect(await ui.find({ type: 'Text', text: /^Moved by Downshift$/ })).toBeDefined()
    expect(await ui.find({ type: 'Text', text: /9\.0k tokens · 1 agent\b/ })).toBeDefined()
    await ui.unmount()
  }
})

test('the band names the running task only where it fits', async ($, on) => {
  withDownshift(on)
  on('agent.spawn', (_, e) => ({ model: 'claude-haiku-5-5', agentId: `agent-${e.tool_use_id}` }))
  on('ui.render', ($, e) => {
    const { Text } = $.ui.resolve(e)
    return <Text>engine</Text>
  })
  await $.command.run(SUMMARY)
  await spawnTwo($)

  let ui = await $.ui.mount(band(120, 10))
  expect(await ui.find({ type: 'Text', text: /↳ task a2 \+1/ })).toBeDefined()
  await ui.unmount()
  ui = await $.ui.mount(band(90, 10))
  expect(await ui.findAll({ type: 'Text', text: /↳/ })).toHaveLength(0)
  await ui.unmount()
})

test('the progress bar keeps 2 px from the NOW row, and only when a task is running', async ($, on) => {
  const clock = withDownshift(on)
  on('tool.call', (_, e) => {
    if (e.tool === 'TaskCreate') {
      return { result: { task: { id: e.subject, subject: e.subject } } }
    }
    return { result: { success: true } }
  })
  await $.command.run(SUMMARY)
  await $.tool.call({ tool: 'TaskCreate', subject: '1', description: 'x' })
  await $.tool.call({ tool: 'TaskCreate', subject: '2', description: 'x' })
  await $.tool.call({ tool: 'TaskUpdate', taskId: '1', status: 'completed' })
  await clock.advance(GLIDE_MS)

  const idle = await $.ui.mount(pane('desktop', 60))
  expect((await idle.findAll({ type: 'Svg' })).some(s => s.props.alt === 'bar gap')).toBe(false)
  await idle.unmount()

  await $.tool.call({ tool: 'TaskUpdate', taskId: '2', status: 'in_progress' })
  const ui = await $.ui.mount(pane('desktop', 60))
  const art = await ui.findAll({ type: 'Svg' })
  const at = art.findIndex(s => s.props.alt === '1 of 2 tasks done')
  expect(at).toBeGreaterThanOrEqual(0)
  expect(art[at + 1]?.props.alt).toBe('bar gap')
  expect(art[at + 1]?.props.height).toBe(2)
  expect(await ui.find({ type: 'Text', text: /NOW/ })).toBeDefined()
  await ui.unmount()
})
