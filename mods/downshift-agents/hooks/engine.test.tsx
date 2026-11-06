// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0
//
// Engine-level tests: events go through the mod's hooks, and the hooks
// declared here stand for the engine beneath it (a `$` call is answered
// `{ value }`, an event by its own result). No live session runs here.

import { expect, mock, test } from 'claude-code/testing'

import { PANEL_JSON } from './features/downshift/panel.fixture'
import { GLIDE_MS } from './shared/glide/glide'

const ORIGIN = { plugin: 'engine', tier: 'core' } as const

const UNCUT = { isStdoutTruncated: false, isStderrTruncated: false }

const SUMMARY = {
  command: 'ds-agents',
  args: 'summary',
  origin: { kind: 'composer' },
  presentation: { isFullscreen: false, columns: 100 },
} as const

const spawnInput = (id: string, extra: { parentAgentId?: string; model?: string } = {}) => ({
  tool_use_id: id,
  prompt: 'not stored',
  description: `job ${id}`,
  subagentType: 'general-purpose',
  provider: ORIGIN,
  parentModel: 'claude-opus-5-5',
  background: false,
  fork: false,
  ...extra,
})

const turnInput = (agentId: string | undefined, turnId: string, reason: 'answer' | 'aborted' | 'error', model?: string) => ({
  answer: '',
  durationMs: 10,
  isAborted: reason === 'aborted',
  turnId,
  agentId,
  reason,
  usage:
    model === undefined
      ? undefined
      : { model, input_tokens: 1200, output_tokens: 300, cache_read_input_tokens: 0, cache_creation_input_tokens: 0 },
})

const pane = (surface: 'terminal' | 'desktop', bodyColumns = 60) => ({
  plugin: 'downshift-agents',
  component: 'Pane',
  requestId: 'downshift-agents',
  surface,
  props: { title: 'Downshift Agents', isFocused: false, bodyColumns, placement: 'dock', scroll: { offset: 0, bodyRows: 40 }, view: {} },
} as const)

const band = (bodyColumns: number, hasSurvey = false) => ({
  plugin: 'downshift-agents',
  component: 'AbovePrompt',
  surface: 'terminal',
  props: { hasSurvey, isWorking: true, maxRows: 3, bodyColumns, scroll: { offset: 0, bodyRows: 3 }, view: {} },
} as const)

const STORED = {
  agents: [],
  board: { source: 'none', tasks: [] },
  downshift: { status: 'unknown' },
  view: { isExpanded: false, filter: 'active', isBandHidden: true },
} as const

test('a spawned agent shows in the pane with its observed model once a turn reports it', async ($, on) => {
  on('clock.now', () => ({ value: 1000 }))
  on('session.id', () => ({ value: 'sess-1' }))
  on('process.run', () => ({ value: { exitCode: 0, stdout: PANEL_JSON, stderr: '', ...UNCUT } }))
  on('agent.spawn', (_, e) => ({ model: 'claude-haiku-5-5', agentId: `agent-${e.tool_use_id}` }))
  on('turn.complete', () => ({ text: '' }))

  await $.agent.spawn(spawnInput('toolu_1', { model: 'haiku' }))
  let ui = await $.ui.mount(pane('desktop'))
  expect((await ui.find({ type: 'Text', text: /job toolu_1/ }))?.text).toContain('job toolu_1')
  expect(await ui.find({ type: 'Text', text: /haiku \? · / })).toBeDefined()
  await ui.unmount()

  await $.turn.complete(turnInput('agent-toolu_1', 'u1', 'answer', 'claude-haiku-5-5'))
  ui = await $.ui.mount(pane('desktop'))
  expect(await ui.find({ type: 'Text', text: /SUBAGENTS 0 active · 1 total/ })).toBeDefined()
  await ui.press({ key: 'filter' })
  expect(await ui.find({ type: 'Text', text: /haiku-5-5 ✓ · / })).toBeDefined()
  expect(await ui.find({ type: 'Text', text: /haiku-5-5 \? · / })).toBeUndefined()
  await ui.unmount()
})

test('Downshift missing: the pane says unavailable and shows no Compactor figure', async ($, on) => {
  on('clock.now', () => ({ value: 1000 }))
  on('session.id', () => ({ value: 'sess-2' }))
  on('process.run', () => {
    throw new Error('ENOENT')
  })
  on('command.run', () => ({ text: '' }))

  const out = await $.command.run(SUMMARY)
  expect(out.text).toContain('Downshift: unavailable (downshift is not on PATH or did not answer)')
  const ui = await $.ui.mount(pane('terminal'))
  expect(await ui.find({ type: 'Text', text: /Downshift metrics unavailable/ })).toBeDefined()
  expect(await ui.find({ type: 'Text', text: /^Tokens saved$/ })).toBeUndefined()
  await ui.unmount()
})

test('a non-zero Downshift exit is reported, never read as zeros', async ($, on) => {
  on('clock.now', () => ({ value: 1000 }))
  on('session.id', () => ({ value: 'sess-3' }))
  on('process.run', () => ({ value: { exitCode: 2, stdout: '', stderr: 'boom', ...UNCUT } }))
  on('command.run', () => ({ text: '' }))

  const out = await $.command.run(SUMMARY)
  expect(out.text).toContain('downshift exited 2')
  expect(out.text).not.toContain('Compactor')
})

test('the summary marks the Compactor token figure as an estimate and routing as workload', async ($, on) => {
  on('clock.now', () => ({ value: 1000 }))
  on('session.id', () => ({ value: 'sess-4' }))
  on('process.run', () => ({ value: { exitCode: 0, stdout: PANEL_JSON, stderr: '', ...UNCUT } }))
  on('command.run', () => ({ text: '' }))

  const out = await $.command.run(SUMMARY)
  expect(out.text).toContain('raw 2.7 MB')
  expect(out.text).toContain('tokens saved*')
  expect(out.text).toContain('* estimate: bytes cut ÷ 4')
  expect(out.text).toContain("spares the top model's quota.")
  expect(out.text).not.toContain('all sessions')
})

test('the session id passed to Downshift is this session, so routing stays per session', async ($, on) => {
  const seen: string[][] = []
  on('clock.now', () => ({ value: 1000 }))
  on('session.id', () => ({ value: 'only-this-session' }))
  on('process.run', (_, e) => {
    seen.push([...e.argv])
    return { value: { exitCode: 0, stdout: PANEL_JSON, stderr: '', ...UNCUT } }
  })
  on('command.run', () => ({ text: '' }))

  await $.command.run(SUMMARY)
  expect(seen.at(-1)).toEqual(['downshift', 'session', '--panel', '--session=only-this-session'])
})

test('the band yields to a survey and keeps what another mod draws beneath', async ($, on) => {
  on('clock.now', () => ({ value: 1000 }))
  on('agent.spawn', (_, e) => ({ model: 'claude-haiku-5-5', agentId: `agent-${e.tool_use_id}` }))
  on('ui.render', ($, e) => {
    const { Text } = $.ui.resolve(e)
    return <Text>other mod band</Text>
  })

  await $.agent.spawn(spawnInput('toolu_b'))
  let ui = await $.ui.mount(band(120))
  expect(await ui.find({ type: 'Text', text: /Downshift Agents/ })).toBeDefined()
  expect(await ui.find({ type: 'Text', text: /other mod band/ })).toBeDefined()
  await ui.unmount()

  ui = await $.ui.mount(band(120, true))
  expect(await ui.find({ type: 'Text', text: /Downshift Agents/ })).toBeUndefined()
  expect(await ui.find({ type: 'Text', text: /other mod band/ })).toBeDefined()
  await ui.unmount()
})

test('with zero agents and no data the band draws nothing of its own', async ($, on) => {
  on('ui.render', ($, e) => {
    const { Text } = $.ui.resolve(e)
    return <Text>engine</Text>
  })
  const ui = await $.ui.mount(band(120))
  expect(await ui.find({ type: 'Text', text: /Downshift Agents/ })).toBeUndefined()
  await ui.unmount()
})

test('tasks from the task tools reach the band', async ($, on) => {
  const clock = mock.clock(on, { now: 1000 })
  on('ui.render', ($, e) => {
    const { Text } = $.ui.resolve(e)
    return <Text>engine</Text>
  })
  on('tool.call', (_, e) =>
    e.tool === 'TaskCreate' ? { result: { task: { id: '1', subject: 'Read router' } } } : { result: { success: true } },
  )

  await $.tool.call({ tool: 'TaskCreate', subject: 'Read router', description: 'x' })
  await $.tool.call({ tool: 'TaskUpdate', taskId: '1', status: 'completed' })
  await clock.advance(GLIDE_MS)
  const ui = await $.ui.mount(band(120))
  expect(await ui.find({ type: 'Text', text: /Tasks 1\/1/ })).toBeDefined()
  await ui.unmount()
})

test('a finished task grows the band step by step: the percent climbs, then settles on the live figure', async ($, on) => {
  const clock = mock.clock(on, { now: 1000 })
  on('ui.render', ($, e) => {
    const { Text } = $.ui.resolve(e)
    return <Text>engine</Text>
  })
  on('tool.call', (_, e) =>
    e.tool === 'TaskCreate' ? { result: { task: { id: e.subject, subject: e.subject } } } : { result: { success: true } },
  )
  const percentShown = async (): Promise<number> => {
    const ui = await $.ui.mount(band(120))
    const text = (await ui.find({ type: 'Text', text: /\d+%$/ }))?.text ?? ''
    await ui.unmount()
    return Number(/(\d+)%$/.exec(text)?.[1])
  }

  await $.tool.call({ tool: 'TaskCreate', subject: 'a', description: 'x' })
  await $.tool.call({ tool: 'TaskCreate', subject: 'b', description: 'x' })
  await $.tool.call({ tool: 'TaskUpdate', taskId: 'a', status: 'completed' })
  await $.tool.call({ tool: 'TaskUpdate', taskId: 'b', status: 'completed' })
  const shown = [await percentShown()]
  for (let i = 0; i < 3; i++) {
    await clock.advance(GLIDE_MS / 4)
    shown.push(await percentShown())
  }
  await clock.advance(GLIDE_MS)
  shown.push(await percentShown())

  expect(shown[0]).toBeLessThan(100)
  expect(shown.every((v, i) => i === 0 || v >= shown[i - 1])).toBe(true)
  expect(new Set(shown).size).toBeGreaterThan(2)
  expect(shown.at(-1)).toBe(100)
})

test('a stored view from an earlier load is kept: a hidden band stays hidden', async ($, on) => {
  on('state.get', (_, e) => ({ value: { value: STORED[e.key as keyof typeof STORED], version: 1 } }))
  on('clock.now', () => ({ value: 1000 }))
  on('ui.render', ($, e) => {
    const { Text } = $.ui.resolve(e)
    return <Text>engine</Text>
  })
  on('agent.spawn', (_, e) => ({ model: 'claude-haiku-5-5', agentId: `agent-${e.tool_use_id}` }))

  await $.agent.spawn(spawnInput('toolu_h'))
  const ui = await $.ui.mount(band(120))
  expect(await ui.find({ type: 'Text', text: /Downshift Agents/ })).toBeUndefined()
  await ui.unmount()
})

test('many agents: the pane lists a bounded number and says how many more', async ($, on) => {
  on('clock.now', () => ({ value: 1000 }))
  on('agent.spawn', (_, e) => ({ model: 'claude-haiku-5-5', agentId: `agent-${e.tool_use_id}` }))

  for (let i = 0; i < 45; i++) {
    await $.agent.spawn(spawnInput(`toolu_${i}`))
  }
  const ui = await $.ui.mount(pane('terminal', 40))
  expect(await ui.find({ type: 'Text', text: /15 more/ })).toBeDefined()
  await ui.unmount()
})

test('the main loop turn does not count as an agent', async ($, on) => {
  on('clock.now', () => ({ value: 1000 }))
  on('session.id', () => ({ value: 's' }))
  on('process.run', () => ({ value: { exitCode: 1, stdout: '', stderr: '', ...UNCUT } }))
  on('turn.complete', () => ({ text: '' }))
  on('command.run', () => ({ text: '' }))

  await $.turn.complete(turnInput(undefined, 'main-1', 'answer', 'claude-opus-5-5'))
  const out = await $.command.run(SUMMARY)
  expect(out.text).toContain('Agents: 0 active, 0 total, 0 failed')
})

test('a failed agent turn is counted as failed, not completed', async ($, on) => {
  on('clock.now', () => ({ value: 1000 }))
  on('session.id', () => ({ value: 's' }))
  on('process.run', () => ({ value: { exitCode: 1, stdout: '', stderr: '', ...UNCUT } }))
  on('agent.spawn', (_, e) => ({ model: 'claude-opus-5-5', agentId: `agent-${e.tool_use_id}` }))
  on('turn.complete', () => ({ text: '' }))
  on('command.run', () => ({ text: '' }))

  await $.agent.spawn(spawnInput('toolu_f'))
  await $.turn.complete(turnInput('agent-toolu_f', 'f1', 'error'))
  const out = await $.command.run(SUMMARY)
  expect(out.text).toContain('Agents: 0 active, 1 total, 1 failed')
})

test('a spawned agent opens the pane without focus; a denied one does not', async ($, on) => {
  const opened: string[] = []
  on('clock.now', () => ({ value: 1000 }))
  on('ui.open', (_, e) => {
    opened.push(`${e.id}:${e.focus === true ? 'focus' : 'nofocus'}`)
    return { value: { isPlaced: true } }
  })
  on('agent.spawn', (_, e) =>
    e.tool_use_id === 'toolu_no' ? { deny: 'policy' } : { model: 'claude-haiku-5-5', agentId: `agent-${e.tool_use_id}` },
  )

  await $.agent.spawn(spawnInput('toolu_no'))
  expect(opened).toEqual([])
  await $.agent.spawn(spawnInput('toolu_yes'))
  expect(opened).toEqual(['downshift-agents:nofocus'])
})

const REPORT = { ...SUMMARY, args: 'report' } as const

test('/ds-agents report prints downshift report as Downshift wrote it', async ($, on) => {
  const seen: string[][] = []
  const md = '# Downshift report\n\n| family | avoided tokens |\n| --- | --- |\n| rg | 1200 |'
  on('process.run', (_, e) => {
    seen.push([...e.argv])
    return { value: { exitCode: 0, stdout: `${md}\n`, stderr: '', ...UNCUT } }
  })
  on('command.run', () => ({ text: '' }))

  const out = await $.command.run(REPORT)
  expect(out.text).toBe(md)
  expect(seen).toEqual([['downshift', 'report']])
})

test('/ds-agents report says why when Downshift fails or is missing', async ($, on) => {
  let missing = false
  on('process.run', () => {
    if (missing) {
      throw new Error('ENOENT')
    }
    return { value: { exitCode: 3, stdout: '', stderr: 'boom', ...UNCUT } }
  })
  on('command.run', () => ({ text: '' }))

  expect((await $.command.run(REPORT)).text).toBe('Downshift report unavailable: downshift exited 3.')
  missing = true
  expect((await $.command.run(REPORT)).text).toBe('Downshift report unavailable: downshift is not on PATH or did not answer.')
})
