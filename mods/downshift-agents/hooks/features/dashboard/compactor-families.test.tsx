// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0
//
// The Compactor's "Top families": rows on the terminal card and the desktop
// panel, the summary line, and nothing new when a binary sends no families.
// No live session runs here.

import type { On } from 'claude-code'
import { expect, mock, test } from 'claude-code/testing'

import type { CompactorFamily } from '../../../types'
import { PANEL_JSON } from '../downshift/panel.fixture'
import { familyShare, rowCells } from './compactor-families'

const UNCUT = { isStdoutTruncated: false, isStderrTruncated: false }
const SUMMARY = { command: 'ds-agents', args: 'summary', origin: { kind: 'composer' }, presentation: { isFullscreen: false, columns: 100 } } as const

const pane = (surface: 'terminal' | 'desktop', bodyColumns: number) =>
  ({
    plugin: 'downshift-agents',
    component: 'Pane',
    requestId: 'downshift-agents',
    surface,
    props: { title: 'Downshift Agents', isFocused: false, bodyColumns, placement: 'dock', scroll: { offset: 0, bodyRows: 40 }, view: {} },
  }) as const

const withFamilies = (families: unknown): string => {
  const raw = JSON.parse(PANEL_JSON) as { compactor: Record<string, unknown> }
  if (families === undefined) {
    delete raw.compactor.families
  } else {
    raw.compactor.families = families
  }
  return JSON.stringify(raw)
}

const five = ['rg', 'git-grep', 'go-test', 'tsc', 'eslint'].map((family, i) => ({ family, commands: 3, compacted: 3, raw_tokens: 1000, avoided_tokens: 500 - i * 10 }))

function serve(on: On, stdout: string) {
  mock.clock(on, { now: 1000 })
  on('session.id', () => ({ value: 'families' }))
  on('process.run', () => ({ value: { exitCode: 0, stdout, stderr: '', ...UNCUT } }))
  on('command.run', () => ({ text: '' }))
}

test('the terminal card lists the top families with their percent and estimated tokens', async ($, on) => {
  serve(on, PANEL_JSON)
  await $.command.run(SUMMARY)
  const ui = await $.ui.mount(pane('terminal', 60))
  expect(await ui.find({ type: 'Text', text: /Top families/ })).toBeDefined()
  expect(await ui.find({ type: 'Text', text: /^rg\s*$/ })).toBeDefined()
  expect(await ui.find({ type: 'Text', text: /^\s*42%$/ })).toBeDefined()
  expect(await ui.find({ type: 'Text', text: /^~2\.0k\*$/ })).toBeDefined()
  expect(await ui.find({ type: 'Text', text: /^\s*~800\*$/ })).toBeDefined()
  await ui.unmount()
})

test('desktop shows three families collapsed and five expanded, as drawn bars', async ($, on) => {
  serve(on, withFamilies(five))
  await $.command.run(SUMMARY)
  const ui = await $.ui.mount(pane('desktop', 60))
  const bars = async () => (await ui.findAll({ type: 'Svg' })).filter(s => /of its tokens cut/.test(String(s.props.alt)))
  expect(await ui.find({ type: 'Text', text: /TOP FAMILIES/ })).toBeDefined()
  expect(await bars()).toHaveLength(3)
  expect(await ui.find({ type: 'Text', text: /^rg · 50%$/ })).toBeDefined()
  await ui.press({ key: 'expand' })
  expect(await bars()).toHaveLength(5)
  await ui.unmount()
})

for (const [name, families] of [['missing', undefined], ['empty', []]] as const) {
  test(`${name} families draw nothing new`, async ($, on) => {
    serve(on, withFamilies(families))
    await $.command.run(SUMMARY)
    for (const surface of ['terminal', 'desktop'] as const) {
      const ui = await $.ui.mount(pane(surface, 60))
      await ui.press({ key: 'expand' })
      expect(await ui.findAll({ type: 'Text', text: /top families/i })).toHaveLength(0)
      expect((await ui.findAll({ type: 'Svg' })).filter(s => /of its tokens cut/.test(String(s.props.alt)))).toHaveLength(0)
      await ui.unmount()
    }
  })
}

test('the summary names the top families', async ($, on) => {
  serve(on, PANEL_JSON)
  const out = await $.command.run(SUMMARY)
  expect(out.text).toContain('Top families: rg ~2.0k*, git-grep ~800*, go-test ~120*')
})

test('the summary has no families line without families', async ($, on) => {
  serve(on, withFamilies(undefined))
  expect((await $.command.run(SUMMARY)).text).not.toContain('Top families')
})

test('a family row never exceeds its columns, down to 40 and below', () => {
  const f: CompactorFamily = { family: 'a-very-long-family-name', commands: 1, compacted: 1, rawTokens: 1_000_000, avoidedTokens: 999_000 }
  for (const width of [60, 36, 24, 12]) {
    const nameWidth = Math.min(12, Math.max(4, Math.floor(width / 3)))
    const c = rowCells(f, nameWidth, 8, width)
    const cells = [c.name, c.pct, c.saved, ...(c.bar > 0 ? ['x'.repeat(c.bar)] : [])]
    expect(cells.reduce((n, s) => n + s.length, 0) + cells.length - 1).toBeLessThanOrEqual(Math.max(width, nameWidth + 4 + 8 + 2))
  }
  expect(rowCells(f, 12, 8, 36).bar).toBeGreaterThan(0)
  expect(familyShare({ ...f, rawTokens: 0 })).toBe(0)
})
