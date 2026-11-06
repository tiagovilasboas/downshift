// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0
//
// The main session's sprite: the same figure, no tool, wearing a headset with a
// boom mic, the one who coordinates the rest. Active, it points to delegate and
// its mic lights as it speaks; idle, it stands still.

import { HUE } from '../shapes'
import type { Drawing } from '../shapes'
import { drawLoop } from './agent-sprite'
import { body, eyes, rightHand } from './figure'
import { put, span } from './pixels'
import type { Grid } from './pixels'

/** The figure in its headset; `speaking` lights the mic tip, `pointing` stretches the left arm. */
export function headset(speaking: boolean, pointing: boolean): Grid {
  const g = rightHand(eyes(body()), 8)
  put(g, [...span(3, 8, 2), [2, 3], [9, 3]], 'H')
  put(g, [[1, 4], [1, 5], [1, 6], [2, 4], [2, 5], [2, 6], [9, 4], [9, 5], [9, 6], [10, 4], [10, 5], [10, 6]], 'H')
  put(g, [[1, 5], [10, 5]], 'M')
  put(g, [[10, 7]], 'H')
  put(g, [[9, 8]], speaking ? 'O' : 'H')
  return put(g, pointing ? [[0, 7], [1, 7]] : [[1, 7]], '#')
}

/** A deeper, more saturated orange than the agents' #d97757, so the lead stands out. */
export const LEAD_ORANGE = '#e8590c'

/** The orchestrator, strong orange and moving while it has agents running, gray and still otherwise. */
export function orchestratorSprite(isActive: boolean, size = 28): Drawing {
  const loop = isActive ? { frames: [headset(false, false), headset(true, true)], period: 1.2 } : { frames: [headset(false, false)], period: 0 }
  return drawLoop(loop, { body: isActive ? LEAD_ORANGE : HUE.gray, eye: '#2c2c2a', bubble: '#888780' }, size)
}
