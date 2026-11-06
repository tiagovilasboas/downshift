// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0
//
// The Efficiency card on the terminal and the desktop: per harness, never
// summed, unobserved instead of 0%, no money, no tokens, normal colours.

import { expect, test } from 'claude-code/testing'

import { PANEL_JSON } from '../downshift/panel.fixture'
import { WARN } from '../../shared/ui/format'

const UNCUT = { isStdoutTruncated: false, isStderrTruncated: false }

const pane = (surface: 'terminal' | 'desktop') =>
  ({
    plugin: 'downshift-agents',
    component: 'Pane',
    requestId: 'downshift-agents',
    surface,
    props: { title: 'Downshift Agents', isFocused: false, bodyColumns: 100, placement: 'dock', scroll: { offset: 0, bodyRows: 40 }, view: {} },
  }) as const

const SUMMARY = { command: 'ds-agents', args: 'summary', origin: { kind: 'composer' }, presentation: { isFullscreen: false, columns: 100 } } as const

test('the Efficiency card shows each harness apart, unobserved never as 0%, in normal colours', async ($, on) => {
  on('clock.now', () => ({ value: 1000 }))
  on('session.id', () => ({ value: 'efficiency' }))
  on('process.run', () => ({ value: { exitCode: 0, stdout: PANEL_JSON, stderr: '', ...UNCUT } }))
  on('command.run', () => ({ text: '' }))
  await $.command.run(SUMMARY)

  for (const surface of ['terminal', 'desktop'] as const) {
    const ui = await $.ui.mount(pane(surface))
    if (surface === 'terminal') {
      await ui.press({ key: 'expand' }) // the view is shared: one press expands both surfaces
    }
    expect(await ui.find({ type: 'Text', text: /EFFICIENCY/ })).toBeDefined()
    const premium = await ui.findAll({ type: 'Text', text: /^Premium preserved:/ })
    expect(premium.map(t => t.text)).toEqual([
      'Premium preserved: 75% (3 of 4 completed tasks ran below frontier)',
      'Premium preserved: unobserved',
    ])
    expect(premium.some(t => t.props.color === WARN)).toBe(false)
    expect(await ui.find({ type: 'Text', text: /^small 2 · mid 1 · frontier 1 · unknown 1$/ })).toBeDefined()
    expect(await ui.find({ type: 'Text', text: /^25% \(1 of 4 ended\)$/ })).toBeDefined()
    expect(await ui.find({ type: 'Text', text: /^unobserved \(no spawn ended\)$/ })).toBeDefined()
    expect(await ui.find({ type: 'Text', text: /0% \(0 of/ })).toBeUndefined()
    await ui.unmount()
  }
})

test('without routed spawns the card says so instead of drawing zeros', async ($, on) => {
  const empty = JSON.stringify({ ...JSON.parse(PANEL_JSON), efficiency: { harnesses: [] } })
  on('clock.now', () => ({ value: 1000 }))
  on('session.id', () => ({ value: 'efficiency-empty' }))
  on('process.run', () => ({ value: { exitCode: 0, stdout: empty, stderr: '', ...UNCUT } }))
  on('command.run', () => ({ text: '' }))
  await $.command.run(SUMMARY)

  for (const surface of ['terminal', 'desktop'] as const) {
    const ui = await $.ui.mount(pane(surface))
    expect(await ui.find({ type: 'Text', text: /^No routed spawns in this window\.$/ })).toBeDefined()
    expect(await ui.find({ type: 'Text', text: /^Premium preserved:/ })).toBeUndefined()
    await ui.unmount()
  }
})
