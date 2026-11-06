// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0
//
// The band on the two surfaces that raise it: the desktop draws a logo and a
// bar as Svg beside the same figures as text; the terminal keeps its glyph
// meter and draws no Svg. No live session runs here.

import type { On } from 'claude-code'
import { expect, mock, test } from 'claude-code/testing'
import type { Engine, MockClock } from 'claude-code/testing'

import { PANEL_JSON } from '../downshift/panel.fixture'
import { GLIDE_MS } from '../../shared/glide/glide'
import { groundOf } from '../../shared/ui/ground'

const UNCUT = { isStdoutTruncated: false, isStderrTruncated: false }

const SUMMARY = {
  command: 'ds-agents',
  args: 'summary',
  origin: { kind: 'composer' },
  presentation: { isFullscreen: false, columns: 100 },
} as const

const band = (surface: 'terminal' | 'desktop', bodyColumns: number, maxRows: number) => ({
  plugin: 'downshift-agents',
  component: 'AbovePrompt',
  surface,
  props: { hasSurvey: false, isWorking: true, maxRows, bodyColumns, scroll: { offset: 0, bodyRows: maxRows }, view: {} },
} as const)

/** Downshift answers with the panel fixture; the engine draws a line beneath the band; one of two tasks is done. */
function withSession(on: On): MockClock {
  const clock = mock.clock(on, { now: 1000 })
  on('session.id', () => ({ value: 'band' }))
  on('process.run', () => ({ value: { exitCode: 0, stdout: PANEL_JSON, stderr: '', ...UNCUT } }))
  on('command.run', () => ({ text: '' }))
  on('ui.render', ($, e) => {
    const { Text } = $.ui.resolve(e)
    return <Text>engine</Text>
  })
  on('tool.call', (_, e) => {
    if (e.tool === 'TaskCreate') {
      return { result: { task: { id: e.subject, subject: e.subject } } }
    }
    return { result: { success: true } }
  })
  return clock
}

async function startTasks($: Engine): Promise<void> {
  await $.command.run(SUMMARY)
  await $.tool.call({ tool: 'TaskCreate', subject: '1', description: 'x' })
  await $.tool.call({ tool: 'TaskCreate', subject: '2', description: 'x' })
  await $.tool.call({ tool: 'TaskUpdate', taskId: '1', status: 'completed' })
}

test('a dark or light theme names the ground of the logo; any other leaves it to the OS', () => {
  expect([groundOf('dark'), groundOf('dark-daltonized'), groundOf('light'), groundOf('light-ansi'), groundOf('auto'), groundOf(undefined)]).toEqual([
    'dark',
    'dark',
    'light',
    'light',
    'auto',
    'auto',
  ])
})

const framedBoxes = async (ui: { findAll: (q: { type: string }) => Promise<{ props: Record<string, unknown> }[]> }) =>
  (await ui.findAll({ type: 'Box' })).filter(b => b.props.borderStyle === 'round')

test('the desktop band draws a logo and a task bar as Svg beside the same figures as text', async ($, on) => {
  const clock = withSession(on)
  await startTasks($)
  await clock.advance(GLIDE_MS)

  const ui = await $.ui.mount(band('desktop', 120, 10))
  const art = await ui.findAll({ type: 'Svg' })
  expect(art.every(s => typeof s.props.source === 'string' && s.props.source.startsWith('<svg'))).toBe(true)
  const logo = art.find(s => s.props.alt === 'downshift agents logo')
  expect(logo).toBeDefined()
  expect(String(logo?.props.source)).toMatch(/<path d="M[^"]+"/)
  const meter = art.find(s => s.props.alt === '1 of 2 tasks done')
  expect(meter).toBeDefined()
  expect(meter?.props.width).toBe(64)
  expect(meter?.props.height).toBe(12)

  for (const text of [/Downshift Agents/, /Tasks 1\/2/, /50%/, /Agents 0\/0 active/, /Compact ~14\.4k\*/]) {
    expect(await ui.find({ type: 'Text', text })).toBeDefined()
  }
  expect(await ui.find({ type: 'Text', text: /━/ })).toBeUndefined()
  expect(await ui.find({ type: 'Text', text: /◆/ })).toBeUndefined()
  expect(await framedBoxes(ui)).toHaveLength(0)
  expect(await ui.find({ type: 'Text', text: /engine/ })).toBeDefined()
  await ui.unmount()
})

test('the desktop band never draws a frame, at any size', async ($, on) => {
  withSession(on)
  await startTasks($)

  let ui = await $.ui.mount(band('desktop', 120, 3))
  expect(await framedBoxes(ui)).toHaveLength(0)
  expect(await ui.find({ type: 'Text', text: /Downshift Agents/ })).toBeDefined()
  await ui.unmount()

  ui = await $.ui.mount(band('desktop', 50, 10))
  expect(await framedBoxes(ui)).toHaveLength(0)
  await ui.unmount()
})

test('the desktop band with no task yet draws the text and no bar', async ($, on) => {
  withSession(on)
  await $.command.run(SUMMARY)

  const ui = await $.ui.mount(band('desktop', 120, 10))
  expect(await ui.find({ type: 'Text', text: /Tasks —/ })).toBeDefined()
  const art = await ui.findAll({ type: 'Svg' })
  expect(art.map(s => s.props.alt)).toEqual(['downshift agents logo'])
  await ui.unmount()
})

test('the terminal band keeps its glyph meter and draws no Svg', async ($, on) => {
  const clock = withSession(on)
  await startTasks($)
  await clock.advance(GLIDE_MS)

  const ui = await $.ui.mount(band('terminal', 120, 10))
  expect(await ui.findAll({ type: 'Svg' })).toHaveLength(0)
  expect(await ui.find({ type: 'Text', text: /◆ Downshift Agents/ })).toBeDefined()
  expect(await ui.find({ type: 'Text', text: /Tasks 1\/2 ━+╸?━* 50%/ })).toBeDefined()
  await ui.unmount()
})
