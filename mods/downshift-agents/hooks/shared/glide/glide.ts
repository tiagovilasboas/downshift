// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0
//
// Progressive growth of a shown figure. When a figure moves, the views do not
// jump to it: they show a value that eases from where it stood to the new one
// over GLIDE_MS, read off the clock at each draw. Pure: the clock, the stored
// glides and the redraws live in `ticker.ts`.

import type { Glide, Glides } from '../../../types'

export type { Glide, Glides }

/** How long a figure takes to reach its new value. */
export const GLIDE_MS = 900

/** The figures that glide. */
export type GlideKey = keyof Glides

/** Fast at first, settling softly: cubic ease-out over t in [0, 1]. */
export function easeOut(t: number): number {
  const c = Math.min(1, Math.max(0, t))
  return 1 - Math.pow(1 - c, 3)
}

/** The value to show now: the live value once the glide is over, or with none. */
export function shownValue(glide: Glide | undefined, target: number, now: number): number {
  if (glide === undefined || !Number.isFinite(glide.from) || !Number.isFinite(target)) {
    return target
  }
  const t = (now - glide.at) / GLIDE_MS
  return t >= 1 ? target : glide.from + (target - glide.from) * easeOut(t)
}

/** Whether a glide still moves at `now`. */
export function isMoving(glide: Glide | undefined, now: number): boolean {
  return glide !== undefined && now - glide.at < GLIDE_MS
}

/** The glides still moving at `now`; finished ones are dropped. */
export function moving(glides: Glides, now: number): Glides {
  const out: Glides = {}
  for (const key of Object.keys(glides) as GlideKey[]) {
    if (isMoving(glides[key], now)) {
      out[key] = glides[key]
    }
  }
  return out
}
