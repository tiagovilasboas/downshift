// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0
//
// The figure every sprite starts from: a wide block body (cols 2-9, rows 4-9,
// the last row shaded), four short legs, a left hand at (1,7) that holds the
// tool, and two tall eyes. Nothing sits on the front of the body: whatever an
// agent holds or shows goes beside it or above it.

import { blank, put, span, stamp } from './pixels'
import type { Grid } from './pixels'

export function body(): Grid {
  const g = blank()
  for (let y = 4; y <= 9; y++) put(g, span(2, 9, y), y === 9 ? 'S' : '#')
  for (const x of [2, 4, 7, 9]) put(g, [[x, 10], [x, 11]], 'l')
  return put(g, [[1, 7]], '#')
}

/** Eyes, shifted `dx` toward what the figure looks at; `low` looks down. */
export function eyes(g: Grid, dx = 0, low = false): Grid {
  const cells = low ? [[4 + dx, 6], [7 + dx, 6]] as const : [[4 + dx, 5], [4 + dx, 6], [7 + dx, 5], [7 + dx, 6]] as const
  return put(g, cells, 'k')
}

/** The right hand at row `y`. */
export function rightHand(g: Grid, y: number): Grid {
  return put(g, [[10, y]], '#')
}

/** Both arms raised: the left hand leaves (1,7) for the top. */
export function armsUp(g: Grid): Grid {
  put(g, [[1, 7]], '.')
  return put(g, [[1, 2], [1, 3], [10, 2], [10, 3]], '#')
}

/** Bubble glyphs, three cells wide, drawn at cols 9-11 above the right shoulder. */
const BUBBLES = {
  thinking: ['##.', '..#', '.#.', '...', '.#.'],
  done: ['...', '..#', '..#', '#.#', '.#.'],
  failed: ['.#.', '.#.', '.#.', '...', '.#.'],
} as const

export type Bubble = keyof typeof BUBBLES

export function bubble(g: Grid, kind: Bubble): Grid {
  return stamp(g, BUBBLES[kind], 9, 0, 'b')
}
