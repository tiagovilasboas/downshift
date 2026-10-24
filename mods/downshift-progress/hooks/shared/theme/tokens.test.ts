// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

import { expect, test } from 'claude-code/testing'

import { modeOf, paletteFor, PALETTES } from './tokens'

const HEX = /^#[0-9A-F]{6}$/

function luminance(hex: string): number {
  const [r, g, b] = [1, 3, 5].map(i => parseInt(hex.slice(i, i + 2), 16) / 255)
  const lin = (c: number) => (c <= 0.03928 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4)
  return 0.2126 * lin(r!) + 0.7152 * lin(g!) + 0.0722 * lin(b!)
}

function contrast(a: string, b: string): number {
  const [hi, lo] = [luminance(a), luminance(b)].sort((x, y) => y - x)
  return (hi! + 0.05) / (lo! + 0.05)
}

test('every token is a six-digit hex colour in both modes', () => {
  for (const palette of Object.values(PALETTES)) {
    for (const value of Object.values(palette)) expect(HEX.test(value)).toBe(true)
  }
})

test('text and accent meet WCAG AA (4.5:1) on their own mode background', () => {
  expect(contrast(PALETTES.dark.text, '#0A0A0A')).toBeGreaterThanOrEqual(4.5)
  expect(contrast(PALETTES.dark.accent, '#0A0A0A')).toBeGreaterThanOrEqual(4.5)
  expect(contrast(PALETTES.light.text, '#FFFFFF')).toBeGreaterThanOrEqual(4.5)
  expect(contrast(PALETTES.light.accent, '#FFFFFF')).toBeGreaterThanOrEqual(4.5)
})

test('the theme name picks the mode; no purple accent', () => {
  expect(modeOf('downshift-light')).toBe('light')
  expect(modeOf('light')).toBe('light')
  expect(modeOf('Downshift')).toBe('dark')
  expect(modeOf(undefined)).toBe('dark')
  expect(paletteFor('light')).toBe(PALETTES.light)
  expect(paletteFor('dark')).toBe(PALETTES.dark)
})
