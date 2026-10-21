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
  tokens_line: 'saved 62% of work · 58% of cache',
  compactor: { commands: 4, compacted: 3, raw_bytes: 90000, saved_bytes: 61440, estimated_tokens_saved: 15360, token_estimate: 'estimated: saved_bytes / 4' },
  by_agent_type: [{ agent_type: 'general-purpose', line: 'general-purpose: 62% work · 58% cache', tokens: { work_saved: 620, cache_saved: 58000 } }],
  spawns: [{ tool_use: T1, state: 'switched', from_model: 'claude-opus-5-5', to_model: 'claude-sonnet-5-5', label: 'opus → sonnet (mid)', tokens_label: '48k tokens · saved from opus', tokens: { work_saved: 620, cache_saved: 58000 } }],
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

test('downshift: the row says what happened and what moved down, the header and band stay short', async ($, on) => {
  await world($, on, { stdout: JSON.stringify(PANEL) }, 'S-1')
  expect(await shows($, 'Pane', /opus → sonnet \(mid\) · ↓\d/)).toBe(true)
  expect(await shows($, 'Pane', /^Downshift · 2 switched · 1 held$/)).toBe(true)
  expect(await shows($, 'AbovePrompt', /^Downshift · 2 switched · 1 held$/)).toBe(true)
  // The long sentences of earlier builds are gone.
  expect(await shows($, 'Pane', /by type|saved 62%|work 62%|saved from opus/)).toBe(false)
  expect(await shows($, 'AbovePrompt', /saved 62%|work 62%/)).toBe(false)
})

test('downshift: the Downshift lines carry no dollar figure', async ($, on) => {
  await world($, on, { stdout: JSON.stringify(PANEL) }, 'S-1')
  expect(await shows($, 'AbovePrompt', /Downshift.*\$/)).toBe(false)
  expect(await shows($, 'Pane', /Downshift.*\$|saved from opus.*\$/)).toBe(false)
})

test('downshift: with tokens moved but nothing switched or held the band says Downshift is on', async ($, on) => {
  await world($, on, { stdout: JSON.stringify({ ...PANEL, switched: 0, held: 0 }) }, 'S-1')
  expect(await shows($, 'AbovePrompt', /^Downshift · on$/)).toBe(true)
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
  const clock = mock.clock(on)
  if (!active) on('ui.render', () => ({ type: 'engine' }) as never)
  on('session.id', () => ({ value: 'S-1' }))
  on('ui.open', () => ({ value: { isPlaced: true } }))
  on('process.run', () => ({ value: { exitCode: active ? 0 : 1, stdout: active ? JSON.stringify(PANEL) : '', stderr: '', isStdoutTruncated: false, isStderrTruncated: false } }))
  let n = 0
  on('agent.spawn', () => ({ model: 'claude-sonnet-5-5', agentId: `a${++n}` }))
  for (const id of ['t1', 't2']) {
    await $.agent.spawn({ tool_use_id: id, prompt: 'p', description: `Work ${id}`, subagentType: 'general-purpose', provider: 'engine', parentModel: 'claude-opus-5-5', background: false, fork: false } as never)
  }
  return clock
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
  expect(await shows($, 'AbovePrompt', /Downshift · 2 switched/)).toBe(true)
  expect(argvs.at(-1)).toEqual(['downshift', 'session', '--panel', '--session=S-123'])
})

test('session: with no session id the figure is not drawn (it could be another session\'s)', async ($, on) => {
  const argvs: string[][] = []
  await world($, on, { stdout: JSON.stringify(PANEL) }, '', argvs)
  expect(await shows($, 'AbovePrompt', /Downshift · 2 switched/)).toBe(false)
  expect(argvs.at(-1)).toEqual(['downshift', 'session', '--panel'])
  // The row label is keyed by the spawn's own tool call, so it is still exact.
  expect(await shows($, 'Pane', /opus → sonnet \(mid\)/)).toBe(true)
})

test('band fallback: Downshift inactive or its data missing, the mod draws as upstream (no derived band)', async ($, on) => {
  await spawnTwo($, on, false)
  // Upstream draws nothing here: the engine's own placeholder is all that is left, which the test engine refuses.
  await expect(shows($, 'AbovePrompt', /Subagents|running · /)).rejects.toThrow(/engine drew its own/)
})

// A finished batch stays in the band until a new agent starts, and its band row counts the finished agents.
const runAgents = async ($: Engine, on: On, ids: string[]) => {
  const clock = mock.clock(on, { now: Date.parse('2026-10-19T10:00:00Z') })
  on('session.id', () => ({ value: 'S-1' }))
  on('ui.open', () => ({ value: { isPlaced: true } }))
  on('process.run', () => ({ value: { exitCode: 0, stdout: JSON.stringify(PANEL), stderr: '', isStdoutTruncated: false, isStderrTruncated: false } }))
  let n = 0
  on('agent.spawn', () => ({ model: 'claude-sonnet-5-5', agentId: `a${++n}` }))
  on('turn.complete', () => ({ text: '' }) as never)
  for (const id of ids) {
    await $.agent.spawn({ tool_use_id: id, prompt: 'p', description: `Work ${id}`, subagentType: 'general-purpose', provider: 'engine', parentModel: 'claude-opus-5-5', background: false, fork: false } as never)
  }
  for (let i = 1; i <= ids.length; i++) {
    await ($.turn.complete as (e: unknown) => Promise<unknown>)({ agentId: `a${i}`, reason: 'answer', answer: 'ok' })
  }
  return clock
}

test('band fallback: a finished batch stays in the band and counts its finished agents', async ($, on) => {
  const clock = await runAgents($, on, ['t1', 't2', 't3', 't4', 't5'])
  // Two minutes after the batch ended: no time limit hides it.
  await clock.set(Date.parse('2026-10-19T10:02:00Z'))
  expect(await shows($, 'AbovePrompt', /^Subagents$/)).toBe(true)
  expect(await shows($, 'AbovePrompt', /^5 done$/)).toBe(true)
})

test('band fallback: the finished batch is replaced when a new agent starts', async ($, on) => {
  const clock = await runAgents($, on, ['t1', 't2'])
  expect(await shows($, 'AbovePrompt', /^2 done$/)).toBe(true)
  // The new agent starts well after the finished batch ended.
  await clock.set(Date.parse('2026-10-19T10:05:00Z'))
  await $.agent.spawn({ tool_use_id: 't3', prompt: 'p', description: 'Next', subagentType: 'general-purpose', provider: 'engine', parentModel: 'claude-opus-5-5', background: false, fork: false } as never)
  expect(await shows($, 'AbovePrompt', /^1 running · 0 done$/)).toBe(true)
})

test('band fallback: a finished row leaves the panel 10 seconds after the panel first saw it finished', async ($, on) => {
  const clock = await runAgents($, on, ['t1', 't2'])
  // First rendered at the finish time: still listed nine seconds later, gone after ten.
  expect(await shows($, 'Pane', /Work t1/)).toBe(true)
  await clock.set(Date.parse('2026-10-19T10:00:09Z'))
  expect(await shows($, 'Pane', /Work t1/)).toBe(true)
  await clock.set(Date.parse('2026-10-19T10:00:11Z'))
  expect(await shows($, 'Pane', /Work t1/)).toBe(false)
  // The band still counts the batch.
  expect(await shows($, 'AbovePrompt', /^2 done$/)).toBe(true)
})

// The crew badge counts what the panel lists: running agents, finished ones still inside FINISHED_SHOW_MS, and planned tasks.
const crewLabel = async ($: Engine): Promise<unknown> => {
  const ui = await $.ui.mount({ plugin: PLUGIN, surface: 'terminal', component: 'AbovePrompt', props: band as never })
  const label = (await ui.find({ key: 'savvy-agents' }))?.props.label
  await ui.unmount()
  return label
}

test('crew badge: once the finished rows leave the panel, the badge counts only the running agents', async ($, on) => {
  const clock = await runAgents($, on, ['t1', 't2'])
  // First rendered at the finish time, so the window starts there.
  expect(await crewLabel($)).toBe('×2')
  await $.agent.spawn({ tool_use_id: 't3', prompt: 'p', description: 'Work t3', subagentType: 'general-purpose', provider: 'engine', parentModel: 'claude-opus-5-5', background: false, fork: false } as never)
  await clock.set(Date.parse('2026-10-19T10:00:11Z'))
  expect(await shows($, 'Pane', /Work t1/)).toBe(false)
  expect(await crewLabel($)).toBe('×1')
})

test('crew badge: a finished agent still inside the window is counted with the running ones', async ($, on) => {
  const clock = await runAgents($, on, ['t1', 't2'])
  expect(await crewLabel($)).toBe('×2')
  await $.agent.spawn({ tool_use_id: 't3', prompt: 'p', description: 'Work t3', subagentType: 'general-purpose', provider: 'engine', parentModel: 'claude-opus-5-5', background: false, fork: false } as never)
  await clock.set(Date.parse('2026-10-19T10:00:09Z'))
  expect(await shows($, 'Pane', /Work t1/)).toBe(true)
  expect(await crewLabel($)).toBe('×3')
})

test('downshift: the Compactor shows in its own unit and is never added to the tokens moved', async ($, on) => {
  await world($, on, { stdout: JSON.stringify(PANEL) }, 'S-1')
  expect(await shows($, 'Pane', /59k downshifted · .* · Compactor 4 cmds · 60\.0 KB saved$/)).toBe(true)
  expect(await shows($, 'Pane', /estimated|≈|total|nothing to cut/i)).toBe(false)
})

test('compactor: commands that cut nothing say so, with the bytes they saw', async ($, on) => {
  const quiet = { ...PANEL, compactor: { commands: 4, compacted: 0, raw_bytes: 878, saved_bytes: 0 } }
  await world($, on, { stdout: JSON.stringify(quiet) }, 'S-1')
  expect(await shows($, 'Pane', /Compactor 4 cmds · 0 B saved · nothing to cut$/)).toBe(true)
  expect(await shows($, 'Pane', /↓ 0 B/)).toBe(false)
})

test('compactor: a single command is named in the singular', async ($, on) => {
  const one = { ...PANEL, compactor: { commands: 1, compacted: 0, raw_bytes: 100, saved_bytes: 0 } }
  await world($, on, { stdout: JSON.stringify(one) }, 'S-1')
  expect(await shows($, 'Pane', /Compactor 1 cmd · 0 B saved · nothing to cut$/)).toBe(true)
})

test('compactor: raw output that was compacted is not called nothing to cut', async ($, on) => {
  await world($, on, { stdout: JSON.stringify(PANEL) }, 'S-1')
  expect(await shows($, 'Pane', /nothing to cut/)).toBe(false)
})

test('effort: when effort changed the summary says model and effort, not switched', async ($, on) => {
  const effort = { ...PANEL, model_switched: 1, effort_changed: 2 }
  await world($, on, { stdout: JSON.stringify(effort) }, 'S-1')
  expect(await shows($, 'AbovePrompt', /^Downshift · 1 model · 2 effort · 1 held$/)).toBe(true)
  expect(await shows($, 'Pane', /switched/)).toBe(false)
})

test('effort: a zero effort_changed keeps the switched wording', async ($, on) => {
  const zero = { ...PANEL, model_switched: 2, effort_changed: 0 }
  await world($, on, { stdout: JSON.stringify(zero) }, 'S-1')
  expect(await shows($, 'AbovePrompt', /^Downshift · 2 switched · 1 held$/)).toBe(true)
})

test('effort: an older binary without the fields keeps the switched wording', async ($, on) => {
  await world($, on, { stdout: JSON.stringify(PANEL) }, 'S-1')
  expect(await shows($, 'AbovePrompt', /^Downshift · 2 switched · 1 held$/)).toBe(true)
  expect(await shows($, 'AbovePrompt', /effort|model ·/)).toBe(false)
})

test('effort: a non-numeric field is treated as absent', async ($, on) => {
  const junk = { ...PANEL, model_switched: 'x', effort_changed: null }
  await world($, on, { stdout: JSON.stringify(junk) }, 'S-1')
  expect(await shows($, 'AbovePrompt', /^Downshift · 2 switched · 1 held$/)).toBe(true)
})

test('downshift: without a compactor block the header says nothing about it', async ($, on) => {
  const { compactor: _c, ...noCompactor } = PANEL
  await world($, on, { stdout: JSON.stringify(noCompactor) }, 'S-1')
  expect(await shows($, 'Pane', /compactor/)).toBe(false)
})

// Whether the band draws at all: when it draws nothing it hands over to the engine, which the test engine reports as an error, not a failure here.
const bandDrawn = async ($: Engine, text: RegExp): Promise<boolean> => {
  try {
    const ui = await $.ui.mount({ plugin: PLUGIN, surface: 'terminal', component: 'AbovePrompt', props: band as never })
    const found = (await ui.find({ type: 'Text', text })) !== undefined
    await ui.unmount()
    return found
  } catch (err) {
    if (/drew nothing|no implementation for ui\.render/.test(String(err))) return false
    throw err
  }
}

test('dismiss: the band that follows the live subagents stays away after the dismiss, until a new agent starts', async ($, on) => {
  const clock = await spawnTwo($, on)
  expect(await bandDrawn($, /^2 running · 0 done$/)).toBe(true)
  const ui = await $.ui.mount({ plugin: PLUGIN, surface: 'terminal', component: 'AbovePrompt', props: band as never })
  await ui.press({ key: 'savvy-dismiss' } as never)
  await ui.unmount()
  expect(await bandDrawn($, /running/)).toBe(false)
  await clock.advance(1000)
  await $.agent.spawn({ tool_use_id: 't3', prompt: 'p', description: 'Work t3', subagentType: 'general-purpose', provider: 'engine', parentModel: 'claude-opus-5-5', background: false, fork: false } as never)
  expect(await bandDrawn($, /running/)).toBe(true)
})
