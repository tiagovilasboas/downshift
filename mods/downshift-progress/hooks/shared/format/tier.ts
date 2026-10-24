// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

// Catalog tiers (internal/catalog/catalog.json) to the words the band and pane show.
const TIER_WORDS: ReadonlyMap<string, string> = new Map([
  ['small', 'small'],
  ['mid', 'medium'],
  ['frontier', 'frontier'],
])

/** Short label for a catalog tier; an unknown tier reads as empty, never as raw input. */
export function tierWord(tier: string): string {
  return TIER_WORDS.get(tier) ?? ''
}
