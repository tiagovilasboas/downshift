// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0
//
// The dashboard's icons and status marks as Svg: line icons on a 24-unit
// grid, and one mark per task or agent status in
// that status's hue.

import type { AgentStatus, TaskStatus } from '../../../../types'
import { HUE, INK, svgDoc } from './shapes'
import type { Drawing, Hue } from './shapes'

export type IconName = 'agents' | 'layers' | 'trend' | 'code' | 'routing' | 'info' | 'tasks' | 'bot'

const PATHS: Record<IconName, string> = {
  agents: 'M9 11a3.5 3.5 0 1 0 0-7 3.5 3.5 0 0 0 0 7zM2.5 20a6.5 6.5 0 0 1 13 0M16 4.5a3.5 3.5 0 0 1 0 6.5M18.5 14a6 6 0 0 1 3 6',
  layers: 'M12 3 2.5 8.5 12 14l9.5-5.5L12 3zM2.5 13 12 18.5 21.5 13M2.5 17.5 12 23l9.5-5.5',
  trend: 'M3 7l6.5 6.5 4-4L21 17M21 11v6h-6',
  code: 'M8 7l-5 5 5 5M16 7l5 5-5 5M13.5 4.5l-3 15',
  routing: 'M12 7.5v4M12 11.5l-5.5 5M12 11.5l5.5 5M12 7.5a2.5 2.5 0 1 0 0-5 2.5 2.5 0 0 0 0 5zM5.5 21.5a2.5 2.5 0 1 0 0-5 2.5 2.5 0 0 0 0 5zM18.5 21.5a2.5 2.5 0 1 0 0-5 2.5 2.5 0 0 0 0 5z',
  info: 'M12 21a9 9 0 1 0 0-18 9 9 0 0 0 0 18zM12 11v5.5M12 7.5h.01',
  tasks: 'M3.5 6.5l2 2 3.5-3.5M12 7h8.5M3.5 13.5l2 2 3.5-3.5M12 14h8.5M12 20h8.5',
  bot: 'M7 8h10a3 3 0 0 1 3 3v6a3 3 0 0 1-3 3H7a3 3 0 0 1-3-3v-6a3 3 0 0 1 3-3zM12 8V4.5M9.5 13.5h.01M14.5 13.5h.01M10 17h4',
}

function strokes(path: string, hue: Hue): string {
  return `<path d="${path}" fill="none" stroke="${HUE[hue]}" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/>`
}

/** A line icon `size` px square in `hue`. */
export function icon(name: IconName, hue: Hue, size = 16): Drawing {
  const body = `<g transform="scale(${size / 24})">${strokes(PATHS[name], hue)}</g>`
  return { source: svgDoc(size, size, body), width: size, height: size }
}

const MARKS: Record<TaskStatus | AgentStatus, (c: string) => string> = {
  completed: c => `<circle cx="8" cy="8" r="7" fill="${c}"/><path d="M4.8 8.2l2.2 2.2 4.2-4.6" fill="none" stroke="${INK}" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"/>`,
  running: c =>
    `<circle cx="8" cy="8" r="6" fill="none" stroke="${c}" stroke-opacity="0.25" stroke-width="2"/>` +
    `<path d="M8 2a6 6 0 0 1 6 6" fill="none" stroke="${c}" stroke-width="2" stroke-linecap="round">` +
    `<animateTransform attributeName="transform" type="rotate" from="0 8 8" to="360 8 8" dur="1.2s" repeatCount="indefinite"/></path>`,
  stale: c => `<circle cx="8" cy="8" r="6" fill="none" stroke="${c}" stroke-width="1.6" stroke-dasharray="2.4 2"/><path d="M8 4.8V8l2 1.4" fill="none" stroke="${c}" stroke-width="1.6" stroke-linecap="round"/>`,
  pending: c => `<circle cx="8" cy="8" r="6" fill="none" stroke="${c}" stroke-width="1.6"/>`,
  failed: c => `<circle cx="8" cy="8" r="7" fill="${c}"/><path d="M5.5 5.5l5 5M10.5 5.5l-5 5" stroke="${INK}" stroke-width="1.8" stroke-linecap="round"/>`,
  blocked: c => `<circle cx="8" cy="8" r="6" fill="none" stroke="${c}" stroke-width="1.8"/><path d="M4 12 12 4" stroke="${c}" stroke-width="1.8"/>`,
  cancelled: c => `<circle cx="8" cy="8" r="6" fill="none" stroke="${c}" stroke-width="1.6"/><path d="M5.5 8h5" stroke="${c}" stroke-width="1.8" stroke-linecap="round"/>`,
}

/** One status's mark in `hue`, `size` px square; a running mark turns where the surface animates. */
export function statusMark(status: TaskStatus | AgentStatus, hue: Hue, size = 14): Drawing {
  const body = `<g transform="scale(${size / 16})">${MARKS[status](HUE[hue])}</g>`
  return { source: svgDoc(size, size, body), width: size, height: size }
}
