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
  session: 'a1b2c3d4', switched: 2, held: 1, observed: 0, saved_usd: 1.234, has_saved: true, estimated: true,
  spawns: [{ tool_use: T1, state: 'switched', from_model: 'claude-opus-5-5', to_model: 'claude-sonnet-5-5', label: 'opus → sonnet (mid)' }],
}

const world = async ($: Engine, on: On, answer: { stdout?: string; exitCode?: number }) => {
  mock.clock(on)
  on('ui.open', () => ({ value: { isPlaced: true } }))
  on('process.run', () => ({ value: { exitCode: answer.exitCode ?? 0, stdout: answer.stdout ?? '', stderr: '', isStdoutTruncated: false, isStderrTruncated: false } }))
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

test('downshift: the row says what happened to this spawn, the header and band say what it saved', async ($, on) => {
  await world($, on, { stdout: JSON.stringify(PANEL) })
  expect(await shows($, 'Pane', /opus → sonnet \(mid\)/)).toBe(true)
  expect(await shows($, 'Pane', /Downshift saved ≈\$1\.23 \(est\., list price\) · 2 switched · 1 held/)).toBe(true)
  expect(await shows($, 'AbovePrompt', /Downshift saved ≈\$1\.23 \(est\., list price\)/)).toBe(true)
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
