// Downshift local test (not upstream): what the panel and the band draw from
// `downshift session --panel`, and that missing data draws what upstream draws.
import type { On } from 'claude-code'
import { expect, mock, test } from 'claude-code/testing'
import type { Engine } from 'claude-code/testing'

const PLUGIN = 'savvy-progress'
const pane = { title: '', isFocused: false, bodyColumns: 60, placement: 'dock', scroll: { offset: 0, bodyRows: 30 }, view: {} } as const
const band = { hasSurvey: false, isWorking: false, maxRows: 4, bodyColumns: 100, scroll: { offset: 0, bodyRows: 4 }, view: {} }
// sha256('harness-downshift-session-v1:t1'), the key Downshift's ToolUseKey gives tool call id t1.
const T1 = 'c0c017fb725c209fb6150a1a4c15c346e736441dd739277bc61a35b253b22e66'

const PANEL = {
  session: 'a1b2c3d4', switched: 2, held: 1, observed: 0,
  tokens: { children: 2, saved_children: 1, work_tokens: 1000, work_saved: 620, cache_tokens: 100000, cache_saved: 58000 },
  tokens_line: 'tokens saved: work 62% · cache 58% (59k of 101k)',
  by_agent_type: [{ agent_type: 'general-purpose', line: 'general-purpose: work 62% · cache 58% (1 of 2 agents)' }],
  spawns: [{ tool_use: T1, state: 'switched', from_model: 'claude-opus-5-5', to_model: 'claude-sonnet-5-5', label: 'opus → sonnet (mid)', tokens_label: '48k tokens · saved from opus' }],
}

const world = async ($: Engine, on: On, answer: { stdout?: string; exitCode?: number }, sessionId?: string, argvs: string[][] = []) => {
  mock.clock(on)
  if (sessionId !== undefined) on('session.id', () => ({ value: sessionId }))
  on('ui.open', () => ({ value: { isPlaced: true } }))
  on('process.run', (_$, e) => { argvs.push([...e.argv]); return { value: { exitCode: answer.exitCode ?? 0, stdout: answer.stdout ?? '', stderr: '', isStdoutTruncated: false, isStderrTruncated: false } } })
  on('agent.spawn', () => ({ model: 'claude-sonnet-5-5', agentId: 'a1' }))
  await $.tool.call({ tool: 'mcp__savvy-progress__progress', title: 'Ship it', total: 1, phase: 'delegate' } as never)
  await $.agent.spawn({ tool_use_id: 't1', prompt: 'p', description: 'Work', subagentType: 'general-purpose', provider: 'engine', parentModel: 'claude-opus-5-5', background: false, fork: false } as never)
}

const shows = async ($: Engine, component: 'Pane' | 'AbovePrompt', text: RegExp) => {
  const ui =
    component === 'Pane'
      ? await $.ui.mount({ plugin: PLUGIN, surface: 'terminal', component, requestId: 'savvy-agents', props: pane })
      : await $.ui.mount({ plugin: PLUGIN, surface: 'terminal', component, props: band as never })
  const found = (await ui.find({ type: 'Text', text })) !== undefined
  await ui.unmount()
  return found
}

test('downshift: the row says what happened to this spawn, the header and band say the tokens it saved', async ($, on) => {
  await world($, on, { stdout: JSON.stringify(PANEL) }, 'S-1')
  expect(await shows($, 'Pane', /opus → sonnet \(mid\) · 48k tokens · saved from opus/)).toBe(true)
  expect(await shows($, 'Pane', /Downshift tokens saved: work 62% · cache 58% \(59k of 101k\) · 2 switched · 1 held/)).toBe(true)
  expect(await shows($, 'Pane', /by agent type: general-purpose: work 62% · cache 58% \(1 of 2 agents\)/)).toBe(true)
  expect(await shows($, 'AbovePrompt', /Downshift tokens saved: work 62% · cache 58%/)).toBe(true)
})

test('downshift: the Downshift lines carry no dollar figure', async ($, on) => {
  await world($, on, { stdout: JSON.stringify(PANEL) }, 'S-1')
  expect(await shows($, 'AbovePrompt', /Downshift.*\$/)).toBe(false)
  expect(await shows($, 'Pane', /Downshift.*\$|saved from opus.*\$/)).toBe(false)
})

test('downshift: before any agent ended the band says Downshift is on, with no figure', async ($, on) => {
  await world($, on, { stdout: JSON.stringify({ ...PANEL, tokens_line: '', by_agent_type: [] }) }, 'S-1')
  expect(await shows($, 'AbovePrompt', /^Downshift on · 2 switched · 1 held$/)).toBe(true)
})

for (const [name, answer] of [
  ['a failing binary', { exitCode: 1, stdout: '' }],
  ['garbled output', { stdout: 'not json' }],
  ['output of the wrong shape', { stdout: '{"spawns":"x"}' }],
] as const) {
  test(`downshift: ${name} draws nothing extra`, async ($, on) => {
    await world($, on, answer)
    expect(await shows($, 'Pane', /Downshift|→/)).toBe(false)
    expect(await shows($, 'AbovePrompt', /Downshift/)).toBe(false)
  })
}

// With no flow registered the band follows the live subagents; a registered flow keeps upstream drawing.
const spawnTwo = async ($: Engine, on: On, active = true) => {
  mock.clock(on)
  if (!active) on('ui.render', () => ({ type: 'engine' }) as never)
  on('session.id', () => ({ value: 'S-1' }))
  on('ui.open', () => ({ value: { isPlaced: true } }))
  on('process.run', () => ({ value: { exitCode: active ? 0 : 1, stdout: active ? JSON.stringify(PANEL) : '', stderr: '', isStdoutTruncated: false, isStderrTruncated: false } }))
  let n = 0
  on('agent.spawn', () => ({ model: 'claude-sonnet-5-5', agentId: `a${++n}` }))
  for (const id of ['t1', 't2']) {
    await $.agent.spawn({ tool_use_id: id, prompt: 'p', description: `Work ${id}`, subagentType: 'general-purpose', provider: 'engine', parentModel: 'claude-opus-5-5', background: false, fork: false } as never)
  }
}

test('band fallback: no registered flow, the band shows the running subagents', async ($, on) => {
  await spawnTwo($, on)
  expect(await shows($, 'AbovePrompt', /^Subagents$/)).toBe(true)
  expect(await shows($, 'AbovePrompt', /^2 running · 0 done$/)).toBe(true)
})

test('band fallback: a flow with total 0 is replaced the same way', async ($, on) => {
  await spawnTwo($, on)
  await $.tool.call({ tool: 'mcp__savvy-progress__progress', title: 'Empty plan', phase: 'delegate' } as never)
  expect(await shows($, 'AbovePrompt', /^2 running · 0 done$/)).toBe(true)
})

test('band fallback: a registered flow with a total keeps upstream drawing', async ($, on) => {
  await spawnTwo($, on)
  await $.tool.call({ tool: 'mcp__savvy-progress__progress', title: 'Ship it', total: 4, done: 1, phase: 'delegate' } as never)
  expect(await shows($, 'AbovePrompt', /^Tasks 1\/4$/)).toBe(true)
  expect(await shows($, 'AbovePrompt', /running · /)).toBe(false)
})

test('session: the mod asks Downshift for its own session and draws the figure', async ($, on) => {
  const argvs: string[][] = []
  await world($, on, { stdout: JSON.stringify(PANEL) }, 'S-123', argvs)
  expect(await shows($, 'AbovePrompt', /Downshift tokens saved/)).toBe(true)
  expect(argvs.at(-1)).toEqual(['downshift', 'session', '--panel', '--session=S-123'])
})

test('session: with no session id the figure is not drawn (it could be another session\'s)', async ($, on) => {
  const argvs: string[][] = []
  await world($, on, { stdout: JSON.stringify(PANEL) }, '', argvs)
  expect(await shows($, 'AbovePrompt', /Downshift tokens saved/)).toBe(false)
  expect(argvs.at(-1)).toEqual(['downshift', 'session', '--panel'])
  // The row label is keyed by the spawn's own tool call, so it is still exact.
  expect(await shows($, 'Pane', /opus → sonnet \(mid\)/)).toBe(true)
})

test('band fallback: Downshift inactive or its data missing, the mod draws as upstream (no derived band)', async ($, on) => {
  await spawnTwo($, on, false)
  // Upstream draws nothing here: the engine's own placeholder is all that is left, which the test engine refuses.
  await expect(shows($, 'AbovePrompt', /Subagents|running · /)).rejects.toThrow(/engine drew its own/)
})
