// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0
//
// What an agent does while it waits: one activity per agent, picked by a hash
// of its id so it never changes between renders. The tool stays in the left
// hand; the right hand and the eyes do the activity. Frames are slow, so a
// waiting agent never pulls the eye more than a working one.

import { armsUp, body, bubble, eyes, rightHand } from './figure'
import { put, span } from './pixels'
import type { Grid } from './pixels'
import { holdTool } from './tools'
import type { Kind } from './tools'

/** The frames of an activity, and how long one cycle takes, in seconds. */
export type Loop = { frames: Grid[]; period: number }

type Activity = { play: (kind: Kind | undefined) => Loop }

const at = (kind: Kind | undefined, dx = 0, low = false): Grid => holdTool(eyes(body(), dx, low), kind)

/** A mug in the right hand, its steam drifting. */
function coffee(kind: Kind | undefined): Loop {
  const frame = (steam: readonly (readonly [number, number])[]) => {
    const g = rightHand(at(kind, 1), 8)
    put(g, [[10, 6], [11, 6], [10, 7], [11, 7]], 'C')
    return put(g, steam, 's')
  }
  return { frames: [frame([[10, 4], [11, 3]]), frame([[11, 4], [10, 3]])], period: 2.4 }
}

/** A question in a bubble above the shoulder, there two beats out of three. */
function thinking(kind: Kind | undefined): Loop {
  const on = () => bubble(rightHand(at(kind, 1), 7), 'thinking')
  return { frames: [on(), on(), rightHand(at(kind, 1), 7)], period: 3.6 }
}

/** A log held in the right hand, beside the body, its lines scrolling. */
function readingLog(kind: Kind | undefined): Loop {
  const frame = (line: number) => {
    const g = at(kind, 1, true)
    put(g, [...span(10, 11, 5), ...span(10, 11, 6), ...span(10, 11, 7), ...span(10, 11, 8)], 'C')
    return put(g, span(10, 11, line), 'z')
  }
  return { frames: [frame(5), frame(7)], period: 2.4 }
}

/** Three dots above the shoulder, lit in turn. */
function waiting(kind: Kind | undefined): Loop {
  const frame = (lit: number) => {
    const g = rightHand(at(kind), 7)
    ;[7, 9, 11].forEach((x, i) => put(g, [[x, 2]], i === lit ? 'b' : 'd'))
    return g
  }
  return { frames: [frame(0), frame(1), frame(2)], period: 3.6 }
}

/** Arms up, tool and all, then back down. */
function stretching(kind: Kind | undefined): Loop {
  const up = holdTool(armsUp(eyes(body(), 0, true)), kind, -2)
  return { frames: [rightHand(at(kind), 7), up], period: 4.8 }
}

export const ACTIVITIES: readonly Activity[] = [
  { play: coffee },
  { play: thinking },
  { play: readingLog },
  { play: waiting },
  { play: stretching },
]
