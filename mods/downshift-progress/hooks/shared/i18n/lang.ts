// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

export type Lang = 'en' | 'pt-BR'

/**
 * Picks the language from candidates in priority order (plugin option, Claude Code
 * `language`, then the locale env). The caller reads them; this stays pure.
 * Any `pt*` tag is Brazilian Portuguese (the only Portuguese table); no match is English.
 */
export function resolveLang(candidates: readonly (string | undefined)[]): Lang {
  for (const candidate of candidates) {
    const tag = (candidate ?? '').trim().toLowerCase()
    if (tag.startsWith('pt')) return 'pt-BR'
    if (tag.startsWith('en')) return 'en'
  }
  return 'en'
}
