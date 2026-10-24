// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

export type RowStatus = 'running' | 'finished' | 'failed' | 'planned'

const GLYPHS: Readonly<Record<RowStatus, { unicode: string; ascii: string }>> = {
  running: { unicode: '●', ascii: '*' },
  finished: { unicode: '✓', ascii: '+' },
  failed: { unicode: '✗', ascii: 'x' },
  planned: { unicode: '○', ascii: '-' },
}

/** One status glyph; `ascii` picks the fallback for terminals without Unicode. */
export function statusGlyph(status: RowStatus, ascii = false): string {
  const glyph = GLYPHS[status]
  return ascii ? glyph.ascii : glyph.unicode
}

/**
 * True when the terminal should get ASCII glyphs: `TERM=dumb`, or a locale is set
 * and it is not UTF-8. An unset locale keeps Unicode.
 */
export function prefersAsciiGlyphs(term: string | undefined, locale: string | undefined): boolean {
  if (term === 'dumb') return true
  if (locale === undefined || locale === '') return false
  return !/utf-?8/i.test(locale)
}
