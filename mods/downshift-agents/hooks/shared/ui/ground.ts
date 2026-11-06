// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0
//
// The ground a graphic surface paints, read from the person's theme setting,
// so the bare logo is light on a dark pane and dark on a light one. Shared by
// the band and the pane.

import type { Ground } from './svg/brand'

/**
 * The ground the person's theme paints the pane: a `dark*` or `light*` theme
 * names it; any other (`auto`, a custom theme) leaves it to the OS scheme.
 */
export function groundOf(theme: unknown): Ground {
  if (typeof theme !== 'string') return 'auto'
  if (theme.startsWith('dark')) return 'dark'
  if (theme.startsWith('light')) return 'light'
  return 'auto'
}

/** The ground of the person's theme, or `auto` where the settings cannot be read. */
export async function groundOfSettings($: { config: { list: () => Promise<{ key: string; value: unknown }[]> } }): Promise<Ground> {
  try {
    return groundOf((await $.config.list()).find(row => row.key === 'theme')?.value)
  } catch {
    return 'auto'
  }
}
