// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0
//
// A band line is a list of segments; each names a theme token, never a colour.

import type { Token } from '../../shared/theme/tokens'

export type Segment = { text: string; token: Token; bold?: boolean }
export type Line = readonly Segment[]

export const SEPARATOR: Segment = { text: ' · ', token: 'muted' }

/** Joins non-empty groups of segments with the muted separator. */
export function joinGroups(groups: readonly Line[]): Line {
  const out: Segment[] = []
  for (const group of groups) {
    if (group.length === 0) continue
    if (out.length > 0) out.push(SEPARATOR)
    out.push(...group)
  }
  return out
}
