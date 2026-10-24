// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

import { expect, mock, test } from 'claude-code/testing'
import type { Engine } from 'claude-code/testing'

import { toolUseKey } from '../../shared/roster'
import { PANE_ID } from './command'
import { complete, spawn, world } from './roster-world'

const SURFACES = ['terminal', 'desktop', 'vscode', 'mobile'] as const
const PANE = {
  plugin: 'downshift-progress',
  component: 'Pane',
  requestId: PANE_ID,
  props: { title: 'Subagents', isFocused: false, bodyColumns: 80, placement: 'dock', scroll: { offset: 0, bodyRows: 30 }, view: {} },
} as const

const PANEL = async () =>
  JSON.stringify({
    session: 'abcd1234',
    spawns: [{ tool_use: await toolUseKey('tu-1'), state: 'switched', observed_model: 'claude-haiku-5', saved_short: '58k' }],
    report: { scope: { label: 'last 30 days' }, display: { tokens_saved_short: '74k', scope_label: 'last 30 days' } },
  })

/** The text of every Text in the drawing, in order. */
async function texts($: Engine, surface: (typeof SURFACES)[number]): Promise<{ all: string; buttons: boolean; unmount: () => Promise<void> }> {
  const ui = await $.ui.mount({ ...PANE, surface })
  const all = (await ui.findAll({ type: 'Text' })).map(el => el.text).join('\n')
  return { all, buttons: (await ui.findAll({ type: 'Button' })).length > 0, unmount: () => ui.unmount() }
}

test('empty state when no subagent was ever seen, on every surface', async ($, on) => {
  world($, on)
  for (const surface of SURFACES) {
    const { all, buttons, unmount } = await texts($, surface)
    expect(all).toContain('no subagents yet')
    expect(buttons).toBe(false)
    await unmount()
  }
})

test('Running section: title, tier, model, elapsed, glyph, saved_short; header tile verbatim', async ($, on) => {
  const w = world($, on)
  w.setPanel(await PANEL())
  await $.tool.call({ tool: 'mcp__downshift-progress__progress', title: 'Ship', tasks: [{ title: 'Build it', tier: 'heavy' }, { title: 'Docs', tier: 'light', after: ['Build it'] }] } as never)
  await spawn($, 'tu-1', 'Build it')
  await w.clock.advance(7_000)
  for (const surface of SURFACES) {
    const { all, buttons, unmount } = await texts($, surface)
    expect(all).toContain('Running (1)')
    expect(all).toContain('●')
    expect(all).toContain('Build it')
    expect(all).toContain('heavy · claude-haiku-5 · 7s')
    expect(all).toContain('↓ 58k')
    expect(all).toContain('Tokens saved · last 30 days')
    expect(all).toContain('74k')
    expect(all).toContain('Planned (1)')
    expect(all).toContain('Docs')
    expect(all).toContain('← Build it')
    expect(buttons).toBe(false)
    await unmount()
  }
})

test('Finished and failed rows show, then leave 10 s after they ended', async ($, on) => {
  const w = world($, on)
  await spawn($, 'tu-1', 'good')
  await spawn($, 'tu-2', 'bad')
  await w.clock.advance(4_000)
  await complete($, 'agent-1')
  await complete($, 'agent-2', 'error')
  for (const surface of SURFACES) {
    const { all, unmount } = await texts($, surface)
    expect(all).toContain('Finished (2)')
    expect(all).toContain('✓')
    expect(all).toContain('✗')
    expect(all).not.toContain('Running')
    await unmount()
  }
  await w.clock.advance(10_000)
  const { all, unmount } = await texts($, 'terminal')
  expect(all).toContain('no subagents yet')
  await unmount()
})

test('without the binary the rows draw with no figures and no tile', async ($, on) => {
  const w = world($, on)
  w.setPanel(null)
  await spawn($, 'tu-1', 'Alone')
  const { all, unmount } = await texts($, 'desktop')
  expect(all).toContain('Alone')
  expect(all).not.toContain('Tokens saved')
  expect(all).not.toContain('↓')
  await unmount()
})

test('/downshift-agents opens the pane, and closes it when it is open', async ($, on) => {
  const w = world($, on)
  await $.command.run({ command: 'downshift-agents', args: '' } as never)
  await $.command.run({ command: 'downshift-agents', args: '' } as never)
  expect(w.panes).toEqual([`open:${PANE_ID}`, `close:${PANE_ID}`])
})

test('the pane speaks pt-BR when the locale says so', async ($, on) => {
  const w = world($, on)
  mock.env(on, { LANG: 'pt_BR.UTF-8' })
  w.setPanel(await PANEL())
  await spawn($, 'tu-1', 'Tarefa')
  const { all, unmount } = await texts($, 'terminal')
  expect(all).toContain('Em execução (1)')
  expect(all).toContain('Tokens economizados · last 30 days')
  await unmount()
})
