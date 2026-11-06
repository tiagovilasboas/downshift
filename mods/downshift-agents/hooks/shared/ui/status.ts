// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0
//
// One mark, one color and one hue per status, shared by tasks and agents, so
// the same state reads the same in the band, the task list and the agent list,
// as text (the theme color) or as an Svg (the hue).

import type { AgentStatus, TaskStatus } from '../../../types'
import { ACCENT, DONE, FAIL, MUTED, WARN } from './format'
import type { Hue } from './svg/shapes'

type Look = { mark: string; color: string; hue: Hue; label: string }

const LOOKS: Record<TaskStatus | AgentStatus, Look> = {
  running: { mark: '◐', color: ACCENT, hue: 'green', label: 'running' },
  stale: { mark: '◌', color: WARN, hue: 'orange', label: 'stale' },
  completed: { mark: '✓', color: DONE, hue: 'light', label: 'done' },
  failed: { mark: '✗', color: FAIL, hue: 'red', label: 'failed' },
  blocked: { mark: '⊘', color: WARN, hue: 'orange', label: 'blocked' },
  cancelled: { mark: '–', color: MUTED, hue: 'gray', label: 'cancelled' },
  pending: { mark: '○', color: MUTED, hue: 'gray', label: 'pending' },
}

export function lookOf(status: TaskStatus | AgentStatus): Look {
  return LOOKS[status]
}
