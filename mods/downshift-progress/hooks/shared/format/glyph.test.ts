// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

import { expect, test } from 'claude-code/testing'

import { prefersAsciiGlyphs, statusGlyph } from './glyph'

test('each status has a Unicode glyph and a distinct ASCII fallback', () => {
  expect(statusGlyph('running')).toBe('●')
  expect(statusGlyph('finished')).toBe('✓')
  expect(statusGlyph('failed')).toBe('✗')
  expect(statusGlyph('planned')).toBe('○')
  const ascii = (['running', 'finished', 'failed', 'planned'] as const).map(s => statusGlyph(s, true))
  expect(ascii).toEqual(['*', '+', 'x', '-'])
  expect(ascii.every(g => /^[\x20-\x7e]$/.test(g))).toBe(true)
})

test('ASCII glyphs are chosen for TERM=dumb or a non-UTF-8 locale only', () => {
  expect(prefersAsciiGlyphs('dumb', 'en_US.UTF-8')).toBe(true)
  expect(prefersAsciiGlyphs('xterm-256color', 'C')).toBe(true)
  expect(prefersAsciiGlyphs('xterm-256color', 'pt_BR.UTF-8')).toBe(false)
  expect(prefersAsciiGlyphs('xterm-256color', undefined)).toBe(false)
})
