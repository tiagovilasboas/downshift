// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0
//
// The task progress as the views show it while it glides: the share done eases
// toward the live one, and the count and percent follow it, so the bar and
// the numbers grow together.

import { shownValue } from '../../shared/glide/glide'
import type { Glide } from '../../shared/glide/glide'
import type { Progress } from './board'

/** A progress as drawn: `share` (0..1) is the bar's exact fill. */
export type ShownProgress = Progress & { share: number }

/** The share done of a progress, 0 with no tasks. */
export function shareOf(p: Pick<Progress, 'total' | 'completed'>): number {
  return p.total > 0 ? Math.min(1, Math.max(0, p.completed / p.total)) : 0
}

/** The progress to draw at `now`: the live counts, with the share, count done and percent eased. */
export function shownProgress(p: Progress, glide: Glide | undefined, now: number): ShownProgress {
  const share = Math.min(1, Math.max(0, shownValue(glide, shareOf(p), now)))
  if (p.total === 0) {
    return { ...p, share }
  }
  return { ...p, share, completed: Math.round(share * p.total), percent: Math.round(share * 100) }
}
