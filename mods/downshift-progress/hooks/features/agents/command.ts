// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0
//
// `/downshift-agents`: opens the agents pane, and closes it when it is open.
//
// Downshift by Tiago Vilas Boas · https://github.com/tiagovilasboas/downshift

import type { On } from 'claude-code'
import { t } from '../../shared/i18n/t'

export const PANE_ID = 'downshift-agents'
export const AGENTS_COMMAND = {
  name: 'downshift-agents',
  description: 'Show or hide the Downshift agents pane (running, finished and planned subagents).',
} as const

export function registerAgentsCommand(on: On): void {
  on('command.run', { command: AGENTS_COMMAND.name }, async $ => {
    const title = t('subagents')
    try {
      const isOpen = (await $.ui.panes()).some(pane => pane.id === PANE_ID)
      if (isOpen) {
        await $.ui.close({ id: PANE_ID })
        return { text: `${title} ×` }
      }
      await $.ui.open({ id: PANE_ID, title })
      return { text: title }
    } catch {
      return { text: title }
    }
  })
}
