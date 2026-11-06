// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0
//
// The pose of each state, as frames. Working: eyes on the tool, the tool rising
// one cell with each beat, the right hand alternating. Done and failed stand
// still and say so in a bubble; a failed figure also slumps one cell.

import { body, bubble, eyes, rightHand } from './figure'
import { ACTIVITIES } from './idle'
import type { Loop } from './idle'
import { slump } from './pixels'
import { holdTool } from './tools'
import type { Kind } from './tools'

export type SpriteState = 'work' | 'idle' | 'done' | 'failed'

function working(kind: Kind | undefined): Loop {
  const beat = (lift: number, hand: number) => rightHand(holdTool(eyes(body(), -1), kind, lift), hand)
  return { frames: [beat(0, 7), beat(-1, 6)], period: 0.9 }
}

function done(kind: Kind | undefined): Loop {
  return { frames: [bubble(rightHand(holdTool(eyes(body()), kind), 7), 'done')], period: 0 }
}

/** Slumped, eyes down: the body sinks onto its legs (only the feet row stays), the bubble keeps its place. */
function failed(kind: Kind | undefined): Loop {
  const g = rightHand(holdTool(eyes(body(), 0, true), kind), 8)
  return { frames: [bubble(slump(g, 'l'), 'failed')], period: 0 }
}

/** The pose of `state`; an idle figure plays activity `activity` (any integer, wrapped). */
export function poseOf(state: SpriteState, kind: Kind | undefined, activity = 0): Loop {
  if (state === 'work') return working(kind)
  if (state === 'done') return done(kind)
  if (state === 'failed') return failed(kind)
  const n = ACTIVITIES.length
  return ACTIVITIES[((activity % n) + n) % n]!.play(kind)
}
