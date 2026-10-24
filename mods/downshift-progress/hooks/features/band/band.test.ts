// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

import { expect, mock, test } from 'claude-code/testing'

import { PANEL_NO_REPORT_JSON, PANEL_WITH_NEW_FIELDS_JSON, PANEL_WITHOUT_REPORT_JSON } from '../../shared/data/panel-fixtures'
import { PROGRESS_TOOL } from '../flow'

const BAND = {
  plugin: 'downshift-progress',
  component: 'AbovePrompt',
  props: { hasSurvey: false, isWorking: false, maxRows: 10, bodyColumns: 100, scroll: { offset: 0, bodyRows: 10 }, view: {} },
} as const

const SURFACES = ['terminal', 'desktop'] as const

type Answer = { stdout: string } | Error

/** The engine beneath the band: a clock, an env, `process.run` answering `answer`, and the engine's own band. */
function world(on: any, answer: Answer, env: Record<string, string> = {}) {
  const calls: string[][] = []
  const invalidations: string[] = []
  const clock = mock.clock(on, { now: 1_000_000 })
  mock.env(on, { CLAUDE_CODE_SESSION_ID: 'sess-1', ...env })
  on('process.run', (_: any, e: any) => {
    calls.push([...e.argv])
    if (answer instanceof Error) throw answer
    return { value: { exitCode: 0, stdout: answer.stdout, stderr: '' } }
  })
  on('prompt.submit', (_: any, e: any) => ({ text: e.text }))
  on('ui.invalidate', (_: any, e: any) => {
    invalidations.push(e.event)
    return { value: undefined }
  })
  on('ui.render', ($: any, e: any) => {
    const { Text } = $.ui.resolve(e)
    return Text({ children: 'engine band' })
  })
  return { calls, clock, invalidations }
}

const flow = ($: any, input: Record<string, unknown>) => $.tool.call({ tool: PROGRESS_TOOL, ...input } as never)
/** Submits a prompt so the first fetch lands, then draws the band from the cached panel. */
async function drawn($: any, clock: { settle: () => Promise<void> }, surface: (typeof SURFACES)[number] = 'terminal') {
  await $.prompt.submit({ text: 'go' } as never)
  await clock.settle()
  return $.ui.mount({ ...BAND, surface })
}

const text = async (ui: any, re: RegExp) => (await ui.find({ type: 'Text', text: re }))?.text

test('the flow line shows title, phase, done/total, bar and task glyphs', async ($, on) => {
  world(on, new Error('ENOENT: cannot start'), { LANG: 'en_US.UTF-8' })
  const tasks = [{ title: 'A' }, { title: 'B' }, { title: 'C' }]
  await flow($, { title: 'Ship', phase: 'plan', total: 3, done: 1, tasks })
  for (const surface of SURFACES) {
    const ui = await $.ui.mount({ ...BAND, surface })
    expect(await text(ui, /Ship/)).toBeDefined()
    expect(await text(ui, /1\/3/)).toBeDefined()
    expect(await text(ui, /plan/)).toBeDefined()
    expect(await text(ui, /✓○○/)).toBeDefined()
    await ui.unmount()
  }
  await flow($, { phase: 'review', done: 3 })
  const ui = await $.ui.mount({ ...BAND, surface: 'terminal' })
  expect(await text(ui, /review/)).toBeDefined()
  expect(await text(ui, /3\/3/)).toBeDefined()
  await ui.unmount()
})

test('a finished flow disappears 10 s after it was first drawn finished', async ($, on) => {
  const { clock, invalidations } = world(on, new Error('ENOENT: cannot start'))
  await flow($, { title: 'Ship', total: 2, done: 2, finished: true })
  const ui = await $.ui.mount({ ...BAND, surface: 'terminal' })
  expect(await text(ui, /Ship/)).toBeDefined()
  await clock.advance(9_000)
  await ui.unmount()
  const still = await $.ui.mount({ ...BAND, surface: 'terminal' })
  expect(await text(still, /Ship/)).toBeDefined()
  await still.unmount()
  const before = invalidations.length
  await clock.advance(1_500)
  expect(invalidations.length).toBeGreaterThan(before)
  const gone = await $.ui.mount({ ...BAND, surface: 'terminal' })
  expect(await text(gone, /Ship/)).toBeUndefined()
  expect(await text(gone, /engine band/)).toBeDefined()
  await gone.unmount()
})

test('the summary line prints the binary strings verbatim', async ($, on) => {
  const { clock } = world(on, { stdout: PANEL_WITH_NEW_FIELDS_JSON })
  for (const surface of SURFACES) {
    const ui = await drawn($, clock, surface)
    expect(await text(ui, /Downshift · 1 model · 1 effort/)).toBeDefined()
    expect(await text(ui, /Tokens saved · last 30 days: 73k/)).toBeDefined()
    await ui.unmount()
  }
})

test('an older binary without a summary shows its counts verbatim, zero ones left out', async ($, on) => {
  const { clock } = world(on, { stdout: PANEL_WITHOUT_REPORT_JSON })
  const ui = await drawn($, clock)
  expect(await text(ui, /Downshift · 1 model/)).toBeDefined()
  expect(await text(ui, /effort|Tokens saved/)).toBeUndefined()
  await ui.unmount()
})

test('pt-BR labels follow the locale', async ($, on) => {
  const { clock } = world(on, { stdout: PANEL_WITH_NEW_FIELDS_JSON }, { LANG: 'pt_BR.UTF-8' })
  await flow($, { title: 'Entrega', phase: 'delegate', total: 1 })
  const ui = await drawn($, clock)
  expect(await text(ui, /delegar/)).toBeDefined()
  expect(await text(ui, /Tokens economizados · last 30 days: 73k/)).toBeDefined()
  await ui.unmount()
})

test('nothing to show leaves the band to the engine', async ($, on) => {
  const { clock } = world(on, { stdout: PANEL_NO_REPORT_JSON })
  for (const surface of SURFACES) {
    const ui = await drawn($, clock, surface)
    expect(await text(ui, /engine band/)).toBeDefined()
    await ui.unmount()
  }
})

test('fail-open: no binary means the flow line alone', async ($, on) => {
  const { clock } = world(on, Object.assign(new Error('spawn downshift ENOENT'), { code: 'ENOENT' }))
  await flow($, { title: 'Ship', total: 2, done: 1 })
  const ui = await drawn($, clock)
  expect(await text(ui, /Ship/)).toBeDefined()
  expect(await text(ui, /Tokens saved|Downshift/)).toBeUndefined()
  await ui.unmount()
})

test('the band has no Button', async ($, on) => {
  const { clock } = world(on, { stdout: PANEL_WITH_NEW_FIELDS_JSON })
  await flow($, { title: 'Ship', total: 2, done: 1, tasks: [{ title: 'A' }, { title: 'B' }] })
  for (const surface of SURFACES) {
    const ui = await drawn($, clock, surface)
    expect(await text(ui, /Tokens saved/)).toBeDefined()
    expect(await ui.find({ type: 'Button' })).toBeUndefined()
    await ui.unmount()
  }
})

test('a survey keeps the band to the engine', async ($, on) => {
  world(on, { stdout: PANEL_WITH_NEW_FIELDS_JSON })
  await flow($, { title: 'Ship', total: 2 })
  const ui = await $.ui.mount({ ...BAND, props: { ...BAND.props, hasSurvey: true }, surface: 'terminal' })
  expect(await text(ui, /Ship/)).toBeUndefined()
  await ui.unmount()
})

test('the report is requested first, then --no-report until it is 30 s old', async ($, on) => {
  const { calls, clock } = world(on, { stdout: PANEL_WITH_NEW_FIELDS_JSON })
  const ui = await $.ui.mount({ ...BAND, surface: 'terminal' })
  await $.prompt.submit({ text: 'hi' } as never)
  await clock.advance(1_000)
  await $.prompt.submit({ text: 'again' } as never)
  await clock.advance(31_000)
  await $.prompt.submit({ text: 'later' } as never)
  await clock.settle()
  const noReport = calls.map(argv => argv.includes('--no-report'))
  expect(noReport).toEqual([false, true, false])
  await ui.unmount()
})

test('a flow in progress refreshes every 5 s and stops when it finishes', async ($, on) => {
  const { calls, clock } = world(on, { stdout: PANEL_WITH_NEW_FIELDS_JSON })
  await flow($, { title: 'Ship', total: 2, done: 0 })
  const ui = await $.ui.mount({ ...BAND, surface: 'terminal' })
  await $.prompt.submit({ text: 'go' } as never)
  await clock.settle()
  await clock.advance(10_000)
  expect(calls.length).toBe(3)
  await flow($, { finished: true })
  await clock.advance(5_000)
  const settled = calls.length
  await clock.advance(30_000)
  expect(calls.length).toBe(settled)
  await ui.unmount()
})
