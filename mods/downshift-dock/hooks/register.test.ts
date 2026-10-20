// Copyright (c) 2026 Tiago Vilas Boas. SPDX-License-Identifier: Apache-2.0
//
// Tests for the downshift-dock mod (`claude plugin test mods/downshift-dock`).
// The other mods are stood in for by a `command.list` answer and a recording
// `command.run` hook, so these prove the dock's own contract: which buttons it
// draws and which slash command a press runs. They do not prove that the real
// filetree or savvy-progress panel opens.
//
// Downshift by Tiago Vilas Boas · https://github.com/tiagovilasboas/downshift

import { describe, expect, mock, test } from 'claude-code/testing'
import type { CommandInfo, RenderPropsOf, SessionStartInput } from 'claude-code'

import { ACCENT, IDLE, chips, compactorOn, countRunning, dciMode, detect, dotColor, modeLabel, nextMode, parseMode, visibleTargets } from './register'

const plug = (name: string): CommandInfo => ({ name, description: name, source: 'plugin', plugin: name })
const builtin = (name: string): CommandInfo => ({ name, description: name, source: 'builtin' })

const session: SessionStartInput = { surface: null, isInteractive: false, cwd: '/work' }
const bandProps: RenderPropsOf['AbovePrompt'] = {
  hasSurvey: false, isWorking: false, maxRows: 10, bodyColumns: 75,
  scroll: { offset: 0, bodyRows: 10 }, view: {},
}

const base = (on: any) => {
  mock.clock(on, { now: 1_000_000 })
  on('session.start', (_$: any, e: any) => ({ cwd: e.cwd }))
  // Stands for the engine's own band: an empty line the dock draws beneath.
  on('ui.render', ($: any, e: any) => $.ui.resolve(e).Text({ children: [''] }))
}

describe('detection', () => {
  test('a target counts only as a plugin command', () => {
    expect(detect([plug('filetree'), plug('agents-info')])).toEqual({ files: true, agents: true })
    expect(detect([builtin('filetree')])).toEqual({ files: false, agents: false })
    expect(detect([])).toEqual({ files: false, agents: false })
  })

  test('both buttons show until a listing answers, then only the installed ones', () => {
    expect(visibleTargets(null).map(t => t.key)).toEqual(['files', 'agents'])
    expect(visibleTargets({ files: true, agents: false }).map(t => t.key)).toEqual(['files'])
    expect(visibleTargets({ files: false, agents: false })).toEqual([])
  })
})

describe('dot', () => {
  test('green only while a dock panel is open, grey otherwise', () => {
    expect(dotColor([])).toBe(IDLE)
    expect(dotColor(['other'])).toBe(IDLE)
    expect(dotColor(['filetree'])).toBe(ACCENT)
    expect(dotColor(['savvy-agents', 'x'])).toBe(ACCENT)
    expect(IDLE).toBe('#5C6168')
  })
})

describe('band', () => {
  for (const surface of ['terminal', 'desktop'] as const) {
    test(`${surface}: a press runs the other mod's own command`, async ($, on) => {
      base(on)
      const ran: { command: string; args: string; origin: unknown }[] = []
      on('command.list', () => ({ value: [plug('filetree'), plug('agents-info')] }))
      on('command.run', (_$, e) => {
        ran.push({ command: e.command, args: e.args, origin: e.origin })
        return { text: 'ok' }
      })
      await $.session.start({ ...session, surface, isInteractive: true })
      const ui = await $.ui.mount({ plugin: 'downshift-dock', surface, component: 'AbovePrompt', props: bandProps })
      expect(await ui.find({ type: 'Button', key: 'files' })).toBeDefined()
      expect(await ui.find({ type: 'Button', key: 'agents' })).toBeDefined()
      await ui.press({ plugin: 'downshift-dock', key: 'files' })
      await ui.press({ plugin: 'downshift-dock', key: 'agents' })
      expect(ran.map(r => [r.command, r.args])).toEqual([['filetree', ''], ['agents-info', 'open']])
    })
  }

  test('a button for a mod that is not installed is left out', async ($, on) => {
    base(on)
    on('command.list', () => ({ value: [plug('filetree')] }))
    await $.session.start({ ...session, surface: 'terminal', isInteractive: true })
    const ui = await $.ui.mount({ plugin: 'downshift-dock', surface: 'terminal', component: 'AbovePrompt', props: bandProps })
    expect(await ui.find({ type: 'Button', key: 'files' })).toBeDefined()
    expect(await ui.find({ type: 'Button', key: 'agents' })).toBeUndefined()
  })

  test('the status chips show even when no panel mod is installed', async ($, on) => {
    base(on)
    on('command.list', () => ({ value: [] }))
    await $.session.start({ ...session, surface: 'terminal', isInteractive: true })
    const ui = await $.ui.mount({ plugin: 'downshift-dock', surface: 'terminal', component: 'AbovePrompt', props: bandProps })
    expect(await ui.find({ type: 'Button', key: 'dci' })).toBeDefined()
    expect(await ui.find({ type: 'Button', key: 'compactor' })).toBeDefined()
    expect(await ui.find({ type: 'Button', key: 'files' })).toBeUndefined()
  })

  test('a survey on screen hides the band', async ($, on) => {
    base(on)
    on('command.list', () => ({ value: [plug('filetree'), plug('agents-info')] }))
    await $.session.start({ ...session, surface: 'terminal', isInteractive: true })
    const ui = await $.ui.mount({
      plugin: 'downshift-dock', surface: 'terminal', component: 'AbovePrompt', props: { ...bandProps, hasSurvey: true },
    })
    expect(await ui.find({ type: 'Button', key: 'files' })).toBeUndefined()
  })
})

describe('status', () => {
  test('DCI mode comes from the workspace config, off when unreadable', () => {
    expect(dciMode({ mode: 'active' })).toBe('active')
    expect(dciMode({ mode: 'observe' })).toBe('observe')
    expect(dciMode({ mode: 'off' })).toBe('off')
    expect(dciMode({ mode: 'bogus' })).toBe('off')
    expect(dciMode(null)).toBe('off')
  })

  test('compact is on unless the config file says exec false', () => {
    expect(compactorOn({ exec: true })).toBe(true)
    expect(compactorOn({})).toBe(true)
    expect(compactorOn(null)).toBe(true)
    expect(compactorOn({ exec: false })).toBe(false)
  })

  test('a chip is lit only when its feature does work, and the detail says what to run', () => {
    const [dci, compact] = chips({ dci: 'active', compactor: true })
    expect([dci!.on, compact!.on]).toEqual([true, true])
    const [obs] = chips({ dci: 'observe', compactor: false })
    expect(obs!.on).toBe(false)
    expect(obs!.detail).toContain('downshift dci enable active')
    expect(chips(null).map(c => c.on)).toEqual([false, false])
    expect(chips({ dci: 'off', compactor: false })[1]!.detail).toContain('"exec": false')
  })
})

describe('mode', () => {
  test('a valid mode file is read; a missing, invalid or unknown one is performance', () => {
    expect(parseMode('{"mode":"economy"}')).toBe('economy')
    expect(parseMode('{"mode":"performance"}')).toBe('performance')
    expect(parseMode(null)).toBe('performance')
    expect(parseMode('not json')).toBe('performance')
    expect(parseMode('{"mode":"turbo"}')).toBe('performance')
  })

  test('a press switches between the two modes and the label names the current one', () => {
    expect(nextMode('performance')).toBe('economy')
    expect(nextMode('economy')).toBe('performance')
    expect(modeLabel('economy')).toContain('economy')
    expect(modeLabel('performance')).toContain('performance')
  })
})

describe('running count', () => {
  const agent = (status: string) => ({ status })
  const table: [string, { status: string }[], number][] = [
    ['only running agents count', [agent('running'), agent('done'), agent('failed'), agent('running')], 2],
    ['an empty list counts zero', [], 0],
    ['no running agent counts zero', [agent('done'), agent('failed')], 0],
  ]
  for (const [name, agents, want] of table) {
    test(name, () => {
      expect(countRunning(agents)).toBe(want)
    })
  }
})

describe('mode', () => {
  test('a valid mode file is read; a missing, invalid or unknown one is performance', () => {
    expect(parseMode('{"mode":"economy"}')).toBe('economy')
    expect(parseMode('{"mode":"performance"}')).toBe('performance')
    expect(parseMode(null)).toBe('performance')
    expect(parseMode('not json')).toBe('performance')
    expect(parseMode('{"mode":"turbo"}')).toBe('performance')
  })

  test('a press switches between the two modes and the label names the current one', () => {
    expect(nextMode('performance')).toBe('economy')
    expect(nextMode('economy')).toBe('performance')
    expect(modeLabel('economy')).toContain('economy')
    expect(modeLabel('performance')).toContain('performance')
  })
})
