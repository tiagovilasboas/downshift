// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0
//
// The Compactor's percent cut as the views show it: eased toward the live
// figure while it glides, so its bar and its number grow together.

import type { CompactorFigures, DownshiftSnapshot } from '../../../types'
import { shownValue } from '../../shared/glide/glide'
import type { Glide } from '../../shared/glide/glide'
import { reductionPercent } from './normalize'

/** The percent cut to draw at `now`; undefined when Downshift reported none. */
export function shownCut(snapshot: DownshiftSnapshot, glide: Glide | undefined, now: number): number | undefined {
  const pct = snapshot.status === 'ok' && snapshot.compactor !== undefined ? reductionPercent(snapshot.compactor) : undefined
  return pct === undefined ? undefined : Math.round(shownValue(glide, pct, now))
}

/** The plain line for a session that ran commands and cut none; undefined otherwise. No per-reason counts: the panel carries none. */
export function uncutNote(c: CompactorFigures): string | undefined {
  return c.commands > 0 && c.compacted === 0
    ? `${c.commands} runs, none cut: outputs were small, compound, or outside the allowlist`
    : undefined
}
