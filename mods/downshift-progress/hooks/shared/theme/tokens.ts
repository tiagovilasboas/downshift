// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

// Downshift terminal tokens, the same values as mods/downshift-theme/themes/*.json
// and docs/brand/downshift-brand-tokens.json. Black first: text and muted grey carry
// the chrome; green is a small accent.

export type ThemeMode = 'dark' | 'light'
export type Token = 'text' | 'muted' | 'accent' | 'success' | 'warning' | 'danger'
export type Palette = Readonly<Record<Token, string>>

export const PALETTES: Readonly<Record<ThemeMode, Palette>> = {
  dark: {
    text: '#D6DAE2',
    muted: '#868A90',
    accent: '#22C55E',
    success: '#22C55E',
    warning: '#E0B34A',
    danger: '#F87171',
  },
  light: {
    text: '#1F2328',
    muted: '#5B6068',
    accent: '#15803D',
    success: '#15803D',
    warning: '#92400E',
    danger: '#B91C1C',
  },
}

/** `light` when the Claude Code theme name says so (`light`, `downshift-light`); dark otherwise. */
export function modeOf(themeName: string | undefined): ThemeMode {
  return /light/i.test(themeName ?? '') ? 'light' : 'dark'
}

export function paletteFor(themeName: string | undefined): Palette {
  return PALETTES[modeOf(themeName)]
}
