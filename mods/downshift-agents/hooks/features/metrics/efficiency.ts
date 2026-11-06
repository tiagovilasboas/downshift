// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0
//
// Labels Downshift's Efficiency rows for the views, in the words of
// `downshift report` (cmd/downshift/report_efficiency.go). One block per
// harness, never summed; counts and shares only, no tokens and no money. The
// shares are Downshift's; this file only rounds them.

import type { EfficiencyRow } from '../../../types'

/** One harness's lines: `isPremiumObserved` false reads "unobserved", never 0%. */
export type EfficiencyLines = {
  harness: string
  premium: string
  isPremiumObserved: boolean
  tiers: string
  completed: string
  rework: string
}

/** "75% (3 of 4 completed tasks ran below frontier)", or "unobserved"; unknown-tier tasks left out are stated. */
export function premiumText(r: EfficiencyRow): string {
  if (!r.premiumObserved) {
    return r.tierUnknown > 0 ? `unobserved (${r.tierUnknown} completed with unknown tier)` : 'unobserved'
  }
  const head = `${Math.round(r.premiumPct)}% (${r.nonFrontier} of ${r.tierKnown} completed tasks ran below frontier)`
  return r.tierUnknown > 0 ? `${head}; ${r.tierUnknown} with unknown tier left out` : head
}

/** "small 2 · mid 1 · frontier 1 · unknown 1": counts of the harness's routed spawns. */
export function tiersText(r: EfficiencyRow): string {
  const t = r.byTier
  return `small ${t.small} · mid ${t.mid} · frontier ${t.frontier} · unknown ${t.unknown}`
}

export const completedText = (r: EfficiencyRow): string => `${r.completed} of ${r.ended} ended`

/** "25% (1 of 4 ended)", or unobserved when no spawn ended. */
export function reworkText(r: EfficiencyRow): string {
  return r.ended === 0 ? 'unobserved (no spawn ended)' : `${Math.round(r.reworkRatio * 100)}% (${r.reworked} of ${r.ended} ended)`
}

export function efficiencyLines(rows: readonly EfficiencyRow[]): EfficiencyLines[] {
  return rows.map(r => ({
    harness: r.harness,
    premium: premiumText(r),
    isPremiumObserved: r.premiumObserved,
    tiers: tiersText(r),
    completed: completedText(r),
    rework: reworkText(r),
  }))
}
