// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0
//
// The pixel grid a sprite is drawn on: 12x12 cells, one character per cell.
// '.' is empty; every other character names a paint (see paint.ts).

/** Cells per side of the sprite grid. */
export const GRID = 12

export type Grid = string[][]
export type Cell = readonly [x: number, y: number]

/** An empty grid. */
export function blank(): Grid {
  return Array.from({ length: GRID }, () => Array<string>(GRID).fill('.'))
}

/** Sets `cells` to `paint`; cells off the grid are ignored. */
export function put(grid: Grid, cells: readonly Cell[], paint: string): Grid {
  for (const [x, y] of cells) {
    const row = grid[y]
    if (row !== undefined && x >= 0 && x < GRID) row[x] = paint
  }
  return grid
}

/** The cells x0..x1 of row y. */
export function span(x0: number, x1: number, y: number): Cell[] {
  return Array.from({ length: x1 - x0 + 1 }, (_, i) => [x0 + i, y] as const)
}

/** Stamps an art block with its top-left cell at (x0, y0): each character other than '.' is a paint. */
export function stamp(grid: Grid, rows: readonly string[], x0: number, y0: number, as?: string): Grid {
  rows.forEach((row, dy) => {
    for (let dx = 0; dx < row.length; dx++) {
      const c = row[dx]
      if (c !== undefined && c !== '.') put(grid, [[x0 + dx, y0 + dy]], as ?? c)
    }
  })
  return grid
}

/** The figure slumped one cell down. Cells whose paint is in `keep` stay put unless the figure lands on them. */
export function slump(grid: Grid, keep: string): Grid {
  const out = grid.map(row => row.map(c => (keep.includes(c) ? c : '.')))
  for (let y = GRID - 2; y >= 0; y--) {
    grid[y]?.forEach((c, x) => {
      if (c !== '.' && !keep.includes(c)) put(out, [[x, y + 1]], c)
    })
  }
  return out
}

/** A 32-bit FNV-1a hash: stable across runs and platforms. */
export function hash(id: string): number {
  let h = 0x811c9dc5
  for (let i = 0; i < id.length; i++) h = Math.imul(h ^ id.charCodeAt(i), 0x01000193)
  return h >>> 0
}
