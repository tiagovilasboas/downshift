// Copyright (c) 2026 Tiago Vilas Boas. SPDX-License-Identifier: Apache-2.0
//
// downshift-dock: a one-line band above the prompt whose buttons bring back the
// panels that went away: Files (the filetree mod's pane) and Agents (the
// savvy-progress mod's panel). A press runs the other mod's own slash command
// through `$.command.run`, as if the person typed it; the dock opens nothing
// itself and holds no routing logic.
//
// Downshift by Tiago Vilas Boas · https://github.com/tiagovilasboas/downshift

import { atom, read, update } from 'claude-code'
import type { CommandInfo, EngineInterface, Register } from 'claude-code'

import type { Available } from '../types'

// Downshift brand tokens (docs/brand/downshift-brand-tokens.json). Only the two
// that read on a dark and a light terminal; the band sets no background.
export const ACCENT = '#22C55E'
export const MUTED = '#9CA3AF'

export type Target = { key: 'files' | 'agents'; label: string; command: string; args: string; plugin: string }

export const TARGETS: readonly Target[] = [
  { key: 'files', label: 'Files', command: 'filetree', args: '', plugin: 'filetree' },
  { key: 'agents', label: 'Agents', command: 'agents-info', args: 'open', plugin: 'savvy-progress' },
]

const available = atom({ plugin: 'downshift-dock', key: 'available' } as const, null)
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
    // Other mods register their commands in their own session.start; look once more.
    $.clock.after(RECHECK_MS, () => void refresh($))
    return result
  })

  on('turn.complete', async ($, e, next) => {
    await refresh($)
    return next(e)
  })

  on('ui.render', { component: 'AbovePrompt' }, async ($, e, next) => {
    const rest = await next(e)
    if (e.props.hasSurvey) return rest
    const targets = visibleTargets(await read($, available))
    if (targets.length === 0) return rest

    const { Box, Text, Button } = $.ui.resolve(e)
    return (
      <Box flexDirection="column">
        {rest}
        <Box flexDirection="row" columnGap={1}>
          <Text bold color={ACCENT}>downshift</Text>
          {targets.map(t => (
            <Button key={t.key} label={t.label} variant="primary" onPress={() => void press($, t)} />
          ))}
          <Text color={MUTED}>panels</Text>
        </Box>
      </Box>
    )
  })
}
