// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

import type { On } from 'claude-code'

import { PLUGIN, VERSION } from '../../shared/data/plugin'
import { loadedLine } from '../../shared/format/loaded'

export const COMMAND = { name: PLUGIN, description: 'Confirm the downshift-progress mod is loaded.' } as const

export function registerStatus(on: On): void {
  on('command.run', { command: PLUGIN }, () => ({ text: loadedLine(PLUGIN, VERSION) }))
}
