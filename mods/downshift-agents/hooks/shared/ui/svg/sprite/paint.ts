// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0
//
// Turns a grid into Svg rects: each run of one paint on a row is one rect, and
// rects of one paint share a group with its fill. Tool colours are fixed and
// read on light and dark grounds; the body, eye and bubble colours come per sprite.

import { INK } from '../shapes'
import type { Grid } from './pixels'

/** Tool paints: graphite, dark graphite, metal, wood, blue, green, amber, paper. */
export const TOOL_PAINT: Readonly<Record<string, string>> = {
  G: '#5F5E5A',
  H: '#444441',
  M: '#B4B2A9',
  Y: '#BA7517',
  B: '#378ADD',
  V: '#639922',
  O: '#EF9F27',
  C: '#F1EFE8',
}

/** Bubble and hint gray: mid-tone, so it reads on either theme. */
export const HINT = '#888780'

/** The colours of one sprite. */
export type Colours = { body: string; eye: string; bubble: string }

type Fill = { fill: string; opacity?: number }

/** The fills of each paint, painted in this order (the shadow is the body plus an ink veil). */
function fillsOf(c: Colours): [string, Fill[]][] {
  return [
    ['#', [{ fill: c.body }]],
    ['S', [{ fill: c.body }, { fill: INK, opacity: 0.18 }]],
    ['l', [{ fill: c.body, opacity: 0.75 }]],
    ['k', [{ fill: c.eye }]],
    ['b', [{ fill: c.bubble }]],
    ['d', [{ fill: HINT, opacity: 0.35 }]],
    ['s', [{ fill: HINT, opacity: 0.7 }]],
    ['z', [{ fill: INK, opacity: 0.45 }]],
    ...Object.entries(TOOL_PAINT).map(([k, fill]): [string, Fill[]] => [k, [{ fill }]]),
  ]
}

/** The rects of every run of `paint` in the grid. */
function runs(grid: Grid, paint: string): string {
  let out = ''
  grid.forEach((row, y) => {
    let start = -1
    for (let x = 0; x <= row.length; x++) {
      const on = x < row.length && row[x] === paint
      if (on && start < 0) start = x
      if (!on && start >= 0) {
        out += `<rect x="${start}" y="${y}" width="${x - start}" height="1"/>`
        start = -1
      }
    }
  })
  return out
}

/** The markup of one grid: one group per paint that the grid uses. */
export function paint(grid: Grid, colours: Colours): string {
  let out = ''
  for (const [key, fills] of fillsOf(colours)) {
    const rects = runs(grid, key)
    if (rects === '') continue
    for (const f of fills) out += `<g fill="${f.fill}"${f.opacity === undefined ? '' : ` fill-opacity="${f.opacity}"`}>${rects}</g>`
  }
  return out
}
