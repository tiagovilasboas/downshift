// Copyright (c) 2026 Tiago Vilas Boas. SPDX-License-Identifier: Apache-2.0
//
// downshift-dock: a one-line band above the prompt, right aligned. Two icon
// buttons bring back panels that went away: Files (the filetree mod's pane) and
// Agents (the savvy-progress mod's panel); a press runs the other mod's own slash
// command through `$.command.run`, as if the person typed it. Two status chips
// show whether DCI and the native compactor are on; a press on a chip toasts what
// it means. The dock opens nothing itself and holds no routing logic.
//
// Downshift by Tiago Vilas Boas · https://github.com/tiagovilasboas/downshift

import { atom, read, update } from 'claude-code'
import type { CommandInfo, EngineInterface, Register } from 'claude-code'

import type { Available, DciMode, Mode, Status } from '../types'

// Downshift brand tokens (docs/brand/downshift-brand-tokens.json). Black first:
// text follows the terminal theme, grey carries the chrome, green is a small
// status dot that shows only while a panel is open. The band sets no background.
export const ACCENT = '#22C55E'
export const IDLE = '#5C6168'

export type Target = { key: 'files' | 'agents'; label: string; icon: string; command: string; args: string; plugin: string; pane: string }

export const TARGETS: readonly Target[] = [
  { key: 'files', label: 'Files', icon: '\u25A4', command: 'filetree', args: '', plugin: 'filetree', pane: 'filetree' },
  { key: 'agents', label: 'Agents', icon: '\u25C8', command: 'agents-info', args: 'open', plugin: 'savvy-progress', pane: 'savvy-agents' },
]

const available = atom({ plugin: 'downshift-dock', key: 'available' } as const, null)
const statusAtom = atom({ plugin: 'downshift-dock', key: 'status' } as const, null)
const modeAtom = atom({ plugin: 'downshift-dock', key: 'mode' } as const, null)
const RECHECK_MS = 2000

/** Which target commands the session lists as a plugin's command. */
export function detect(list: readonly CommandInfo[]): Available {
  const has = (t: Target) => list.some(c => c.name === t.command && c.source === 'plugin')
  return { files: has(TARGETS[0]!), agents: has(TARGETS[1]!) }
}

/** The buttons to draw: both while detection has not answered, else the installed ones. */
export function visibleTargets(a: Available | null): Target[] {
  return TARGETS.filter(t => a === null || a[t.key])
}

/** Green means live: the dot is green while at least one of the dock's panels is open, else grey. */
export function dotColor(paneIds: readonly string[]): string {
  return TARGETS.some(t => paneIds.includes(t.pane)) ? ACCENT : IDLE
}

/** The mode file's value. Missing or unknown counts as performance, as in core. */
export function parseMode(text: string | null): Mode {
  try {
    const mode = (JSON.parse(text ?? '') as { mode?: unknown } | null)?.mode
    return mode === 'economy' ? 'economy' : 'performance'
  } catch {
    return 'performance'
  }
}

export function nextMode(m: Mode): Mode {
  return m === 'economy' ? 'performance' : 'economy'
}

export function modeLabel(m: Mode): string {
  return m === 'economy' ? '\u25C6 economy' : '\u25C7 performance'
}

/** The mode file lives in the state dir: DOWNSHIFT_STATE_DIR, else ~/.downshift. */
async function modePath($: EngineInterface): Promise<string> {
  const dir = (await $.env.get('DOWNSHIFT_STATE_DIR')) || `${(await $.env.get('HOME')) ?? ''}/.downshift`
  return `${dir.replace(/\/+$/, '')}/mode.json`
}

async function readModeText($: EngineInterface): Promise<string | null> {
  try {
    return String(await $.fs.read(await modePath($)))
  } catch {
    return null
  }
}

async function refreshMode($: EngineInterface): Promise<void> {
  const next = parseMode(await readModeText($))
  await update($, modeAtom, () => next)
}

async function toggleMode($: EngineInterface): Promise<void> {
  const next = nextMode(parseMode(await readModeText($)))
  try {
    await $.fs.write(await modePath($), JSON.stringify({ mode: next }) + '\n')
    await update($, modeAtom, () => next)
    $.ui.toast(`downshift: ${modeLabel(next)}. Applies to the next subagent spawn.`)
  } catch {
    $.ui.toast('downshift-dock: could not write the mode file. Check the state dir.')
  }
}

/** DCI mode from `<workspace>/.downshift/dci.json`; anything unreadable counts as off. */
export function dciMode(cfg: unknown): DciMode {
  const mode = (cfg as { mode?: unknown } | null)?.mode
  return mode === 'active' || mode === 'observe' ? mode : 'off'
}

/** Compact is always on; only `"exec": false` in `~/.downshift/context-optimization.json` turns it off. */
export function compactorOn(cfg: unknown): boolean {
  const c = cfg as { exec?: unknown } | null
  return c?.exec !== false
}

export type Chip = { key: 'dci' | 'compactor'; label: string; on: boolean; detail: string }

/** The two status chips. A chip is lit (green) when its feature does work for this session. */
export function chips(s: Status | null): Chip[] {
  const dci = s?.dci ?? 'off'
  const compactor = s?.compactor ?? false
  return [
    {
      key: 'dci',
      label: 'DCI',
      on: dci === 'active',
      detail:
        dci === 'active'
          ? 'DCI on: code context is injected into prompts.'
          : dci === 'observe'
            ? 'DCI observing: telemetry only, nothing injected. `downshift dci enable active` turns it on.'
            : 'DCI off. `downshift dci enable active` turns it on.',
    },
    {
      key: 'compactor',
      label: 'Compact',
      on: compactor,
      detail: compactor
        ? 'Compact on: allowlisted shell commands (tests, status) run through `downshift compact` and keep only the useful output.'
        : 'Compact off: `"exec": false` is set in ~/.downshift/context-optimization.json. Remove that line to turn it back on.',
    },
  ]
}

async function readJson($: EngineInterface, path: string): Promise<unknown> {
  try {
    return JSON.parse(String(await $.fs.read(path)))
  } catch {
    return null
  }
}

async function refreshStatus($: EngineInterface): Promise<void> {
  try {
    const home = ((await $.env.get('HOME')) ?? '').replace(/\/+$/, '')
    const cwd = (await $.session.cwd()).replace(/\/+$/, '')
    const next: Status = {
      dci: dciMode(await readJson($, `${cwd}/.downshift/dci.json`)),
      compactor: compactorOn(await readJson($, `${home}/.downshift/context-optimization.json`)),
    }
    await update($, statusAtom, () => next)
  } catch {
    // Keep what is known; the chips show grey until a read answers.
  }
}

/** How many subagents are running in an agent list. */
export function countRunning(agents: readonly { status: string }[]): number {
  return agents.filter(a => a.status === 'running').length
}

/** Subagents whose loop is running now, from the engine's agent list; 0 when it cannot answer. */
async function runningAgents($: EngineInterface): Promise<number> {
  try {
    return countRunning(await $.agent.list())
  } catch {
    return 0
  }
}

async function openPaneIds($: EngineInterface): Promise<string[]> {
  try {
    return (await $.ui.panes()).map(p => p.id)
  } catch {
    return []
  }
}

async function refresh($: EngineInterface): Promise<void> {
  try {
    const next = detect(await $.command.list())
    await update($, available, () => next)
  } catch {
    // Listing failed: keep what is known; the band shows both until a listing answers.
  }
}

async function press($: EngineInterface, t: Target): Promise<void> {
  try {
    await $.command.run({ command: t.command, args: t.args })
  } catch {
    $.ui.toast(`downshift-dock: could not run /${t.command}. Is ${t.plugin}@downshift enabled?`)
  }
}

export const register: Register = on => {
  on('session.start', async ($, e, next) => {
    const result = await next(e)
    await refresh($)
    await refreshStatus($)
    await refreshMode($)
    // Other mods register their commands in their own session.start; look once more.
    $.clock.after(RECHECK_MS, () => void refresh($))
    return result
  })

  on('turn.complete', async ($, e, next) => {
    await refresh($)
    await refreshStatus($)
    await refreshMode($)
    return next(e)
  })

  on('ui.render', { component: 'AbovePrompt' }, async ($, e, next) => {
    const rest = await next(e)
    if (e.props.hasSurvey) return rest
    const targets = visibleTargets(await read($, available))
    const dot = dotColor(await openPaneIds($))
    const status = await read($, statusAtom)
    const mode: Mode = (await read($, modeAtom)) ?? 'performance'
    const running = await runningAgents($)
    const { Box, Text, Button } = $.ui.resolve(e)
    return (
      <Box flexDirection="column">
        {rest}
        <Box flexDirection="row" columnGap={2}>
          <Box flexGrow={1} />
          {running > 0 && <Text color={ACCENT}>{`${running} running`}</Text>}
          <Button key="mode" plain onPress={() => void toggleMode($)}>
            <Text>{modeLabel(mode)}</Text>
          </Button>
          {chips(status).map(c => (
            <Button key={c.key} plain dimColor={!c.on} onPress={() => $.ui.toast(c.detail)}>
              <Text color={c.on ? ACCENT : IDLE}>{'\u25CF'}</Text> {c.label}
            </Button>
          ))}
          {targets.length > 0 && <Text color={dot}>{'\u2502'}</Text>}
          {targets.map(t => (
            <Button key={t.key} plain label={t.icon} onPress={() => void press($, t)} />
          ))}
        </Box>
      </Box>
    )
  })
}
