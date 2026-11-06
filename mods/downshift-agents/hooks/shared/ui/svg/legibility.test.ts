// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0
//
// Legibility of the Svg drawings: a pill and a filled mark carry their own dark
// ground, so they keep their contrast on a light pane as on a dark one, and a
// pill measures its label glyph by glyph, so its text is not stretched.

import { expect, test } from 'claude-code/testing'

import { statusMark } from './icons'
import { HUE, INK, pill, textWidth } from './shapes'
import type { Hue } from './shapes'

const HUES = Object.keys(HUE) as Hue[]
const AA = 4.5

const rgb = (hex: string): number[] => [1, 3, 5].map(i => parseInt(hex.slice(i, i + 2), 16))
const channel = (v: number) => (v / 255 <= 0.03928 ? v / 255 / 12.92 : ((v / 255 + 0.055) / 1.055) ** 2.4)
const luminance = (c: number[]) => 0.2126 * channel(c[0] ?? 0) + 0.7152 * channel(c[1] ?? 0) + 0.0722 * channel(c[2] ?? 0)
const contrast = (a: number[], b: number[]) => {
  const [hi = 0, lo = 0] = [luminance(a), luminance(b)].sort((x, y) => y - x)
  return (hi + 0.05) / (lo + 0.05)
}
/** `fg` at `alpha` over `bg`, as the tint of a hue over the ground. */
const over = (fg: number[], bg: number[], alpha: number) => fg.map((v, i) => v * alpha + (bg[i] ?? 0) * (1 - alpha))

test('a pill draws its own dark ground under the tint, so its label reads on any pane', () => {
  const bad: string[] = []
  for (const hue of HUES) {
    const source = pill('LIVE', hue, { size: 'md' }).source
    const rects = [...source.matchAll(/<rect [^>]*\/>/g)].map(m => m[0])
    const [ground = '', tint = ''] = rects
    const alpha = Number(/fill-opacity="([\d.]+)"/.exec(tint)?.[1])
    if (!ground.includes(`fill="${INK}"`)) bad.push(`${hue}: no ${INK} ground first`)
    if (!tint.includes(`fill="${HUE[hue]}"`) || !(alpha > 0 && alpha < 1)) bad.push(`${hue}: no hue tint over the ground`)
    if (!new RegExp(`<text [^>]*fill="${HUE[hue]}"`).test(source)) bad.push(`${hue}: label is not in its hue`)
    const ratio = contrast(rgb(HUE[hue]), over(rgb(HUE[hue]), rgb(INK), alpha))
    if (!(ratio >= AA)) bad.push(`${hue}: label contrast ${ratio.toFixed(2)}:1 is under ${AA}:1`)
  }
  expect(bad).toEqual([])
})

test('a filled mark draws its check and cross in the dark ink, never white, in every hue', () => {
  const bad: string[] = []
  for (const status of ['completed', 'failed'] as const) {
    for (const hue of HUES) {
      const source = statusMark(status, hue).source
      if (!source.includes(`stroke="${INK}"`)) bad.push(`${status}/${hue}: glyph is not ${INK}`)
      if (/#fff|#ffffff|white/i.test(source)) bad.push(`${status}/${hue}: has a white glyph`)
      const ratio = contrast(rgb(INK), rgb(HUE[hue]))
      if (!(ratio >= AA)) bad.push(`${status}/${hue}: glyph contrast ${ratio.toFixed(2)}:1 is under ${AA}:1`)
    }
  }
  expect(bad).toEqual([])
})

test('the pill measures a glyph by its class: narrow, regular and wide letters are not one width', () => {
  const widths = ['iiii', 'rrrr', 'aaaa', 'AAAA', 'mmmm'].map(t => textWidth(t, 24))
  expect(widths.every((w, i) => i === 0 || w > (widths[i - 1] ?? Infinity))).toBe(true)
  expect(textWidth('LIVE', 12)).toBeLessThan(textWidth('WAIT', 12))
  expect(textWidth('ill', 12)).toBeLessThan(textWidth('abc', 12))
})

// Natural widths of the labels at 12px, weight 600, in the system face, measured
// with canvas measureText in headless Chrome on macOS. The pill sets its text to
// `textLength`, so the ratio is how far the glyphs are stretched (> 1) or squeezed
// (< 1). One width for every letter gave 1.25 for LIVE and 1.25 for failed.
const NATURAL_AT_12PX: Record<string, number> = {
  LIVE: 26.5,
  WAITING: 53.0,
  'N/A': 21.2,
  running: 45.3,
  done: 29.4,
  failed: 32.9,
  blocked: 46.7,
  cancelled: 56.7,
  pending: 48.0,
}

test('a pill label is set within 15% of the width the system face gives it', () => {
  const bad = Object.entries(NATURAL_AT_12PX)
    .map(([label, natural]) => ({ label, ratio: textWidth(label, 12) / natural }))
    .filter(({ ratio }) => ratio < 0.85 || ratio > 1.15)
    .map(({ label, ratio }) => `${label}: ${ratio.toFixed(2)}x`)
  expect(bad).toEqual([])
})
