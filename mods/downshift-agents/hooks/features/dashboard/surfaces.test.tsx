// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0
//
// The pane on every surface: Svg only where the surface draws it, at a finite
// size at any width, and text from tasks and agents only ever in a Text, never
// inside a drawing. No live session runs here.

import { expect, test } from 'claude-code/testing'

import { PANEL_JSON } from '../downshift/panel.fixture'

const UNCUT = { isStdoutTruncated: false, isStderrTruncated: false }
const ORIGIN = { plugin: 'engine', tier: 'core' } as const
const HOSTILE = `<script>alert(1)</script>"&'</text><image href="http://x/y"/>`
const SURFACES = ['terminal', 'desktop', 'vscode', 'mobile'] as const

const pane = (surface: (typeof SURFACES)[number], bodyColumns: number) =>
  ({
    plugin: 'downshift-agents',
    component: 'Pane',
    requestId: 'downshift-agents',
    surface,
    props: { title: 'Downshift Agents', isFocused: false, bodyColumns, placement: 'dock', scroll: { offset: 0, bodyRows: 40 }, view: {} },
  }) as const

type Found = { props: Record<string, unknown> }
const isSize = (v: unknown) => v === undefined || (typeof v === 'number' && Number.isFinite(v) && v > 0)
const isBroken = (s: Found) => {
  const source = String(s.props.source)
  return !isSize(s.props.width) || !isSize(s.props.height) || /NaN|Infinity|undefined/.test(source) || source.length >= 131072
}
const leaks = (s: Found) => /alert\(1\)|<script|<image|href=/i.test(String(s.props.source))

test('only a surface that draws Svg gets it, each at a finite size, and task or agent text never enters a drawing', async ($, on) => {
  on('clock.now', () => ({ value: 1000 }))
  on('session.id', () => ({ value: 'surfaces' }))
  on('process.run', () => ({ value: { exitCode: 0, stdout: PANEL_JSON, stderr: '', ...UNCUT } }))
  on('command.run', () => ({ text: '' }))
  on('agent.spawn', (_, e) => ({ model: 'claude-haiku-5-5', agentId: `agent-${e.tool_use_id}` }))
  let n = 0
  on('tool.call', (_, e) => (e.tool === 'TaskCreate' ? { result: { task: { id: String(++n), subject: HOSTILE } } } : { result: { success: true } }))

  await $.command.run({ command: 'ds-agents', args: 'summary', origin: { kind: 'composer' }, presentation: { isFullscreen: false, columns: 100 } })
  await $.tool.call({ tool: 'TaskCreate', subject: HOSTILE, description: 'x' })
  await $.tool.call({ tool: 'TaskCreate', subject: HOSTILE, description: 'x' })
  await $.tool.call({ tool: 'TaskUpdate', taskId: '1', status: 'in_progress' })
  await $.agent.spawn({ tool_use_id: 't1', prompt: 'x', description: HOSTILE, subagentType: HOSTILE, provider: ORIGIN, parentModel: 'claude-opus-5-5', background: false, fork: false })

  for (const surface of SURFACES) {
    for (const columns of [20, 40, 64, 120, Number.NaN]) {
      const ui = await $.ui.mount(pane(surface, columns))
      await ui.press({ key: 'expand' })
      const art = await ui.findAll({ type: 'Svg' })
      expect(art.length > 0).toBe(surface !== 'terminal')
      expect(art.filter(isBroken)).toHaveLength(0)
      expect(art.filter(leaks)).toHaveLength(0)
      expect(await ui.find({ type: 'Text', text: /alert\(1\)/ })).toBeDefined()
      await ui.unmount()
    }
  }
})
