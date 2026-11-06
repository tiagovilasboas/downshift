// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0
//
// Starts a glide and keeps the views drawing while one moves: one timer for
// the module, a redraw per frame, stopped as soon as every glide has settled.

import type { EngineInterface, Timer } from 'claude-code'

import { readGlides, updateGlides } from '../state/atoms'
import { moving, shownValue } from './glide'
import type { GlideKey } from './glide'

/** One redraw per frame while a figure moves: the desktop's redraw rate. */
const FRAME_MS = 100

// One module load's timer: a reload cancels it with the old environment.
let ticker: Timer | undefined

/**
 * Eases `key` from what the views show now toward `after`. `before` is the
 * figure's live value until this change; a glide already moving starts from
 * the point it reached, so a second change never jumps back.
 */
export async function glideTo($: EngineInterface, key: GlideKey, before: number, after: number): Promise<void> {
  if (before === after || !Number.isFinite(before) || !Number.isFinite(after)) {
    return
  }
  const now = await $.clock.now()
  await updateGlides($, glides => ({ ...glides, [key]: { from: shownValue(glides[key], before, now), at: now } }))
  if (ticker === undefined) {
    ticker = $.clock.every(FRAME_MS, () => void tick($))
  }
}

async function tick($: EngineInterface): Promise<void> {
  const now = await $.clock.now()
  const glides = await readGlides($)
  if (Object.keys(moving(glides, now)).length > 0) {
    $.ui.invalidate('ui.render')
    return
  }
  ticker?.cancel()
  ticker = undefined
  // the last write draws the settled value
  await updateGlides($, () => ({}))
}
