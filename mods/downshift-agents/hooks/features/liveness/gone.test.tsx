// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

import { expect, mock, test } from 'claude-code/testing'

import { PANEL_JSON } from '../downshift/panel.fixture'
import { GONE_AFTER_MS, STALE_AFTER_MS } from '../agents/stale'
import { SWEEP_EVERY_MS } from './sweep'

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

const pane = (surface: 'terminal' | 'desktop') => ({
  plugin: 'downshift-agents',
  component: 'Pane',
  requestId: 'downshift-agents',
  surface,
  props: { title: 'Downshift Agents', isFocused: false, bodyColumns: 60, placement: 'dock', scroll: { offset: 0, bodyRows: 40 }, view: {} },
} as const)

// Regression: a spawn whose end never came stayed in the active view as
// "stale" for the rest of the session, with an idle orchestrator row above it.
for (const surface of ['terminal', 'desktop'] as const) {
  test(`${surface}: a stale agent silent past GONE_AFTER_MS leaves the active view, idle orchestrator included`, async ($, on) => {
    const clock = mock.clock(on, { now: 1000 })
    on('session.id', () => ({ value: 's' }))
    on('process.run', () => ({ value: { exitCode: 0, stdout: PANEL_JSON, stderr: '', ...UNCUT } }))
    on('agent.spawn', (_, e) => ({ model: 'claude-haiku-5-5', agentId: `agent-${e.tool_use_id}` }))

    await $.agent.spawn(spawnInput('t1'))
    await clock.advance(STALE_AFTER_MS + 2 * SWEEP_EVERY_MS)
    let ui = await $.ui.mount(pane(surface))
    expect(await ui.find({ type: 'Text', text: /job t1/ })).toBeDefined()
    await ui.unmount()

    await clock.advance(GONE_AFTER_MS)
    ui = await $.ui.mount(pane(surface))
    expect(await ui.find({ type: 'Text', text: /job t1/ })).toBeUndefined()
    expect(await ui.find({ type: 'Text', text: /^idle$/ })).toBeUndefined()
    expect(await ui.find({ type: 'Text', text: /No agent running now/ })).toBeDefined()
    // still in the full list: nothing was deleted
    await ui.press({ key: 'filter' })
    expect(await ui.find({ type: 'Text', text: /job t1/ })).toBeDefined()
    await ui.unmount()
  })
}
