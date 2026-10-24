// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0
//
// What the band reads from the environment: the language, the glyph set and the
// session id. The engine lists the variables a module reads, so each name is a
// literal at its own call site.

import type { EngineInterface } from 'claude-code'

import { prefersAsciiGlyphs } from '../../shared/format/glyph'
import { resolveLang } from '../../shared/i18n/lang'
import type { Lang } from '../../shared/i18n/lang'

export type Look = { lang: Lang; ascii: boolean }

type Env = Pick<EngineInterface, 'env'>

export async function readLook($: Env): Promise<Look> {
  let all: string | undefined
  let messages: string | undefined
  let lang: string | undefined
  let term: string | undefined
  try {
    all = await $.env.get('LC_ALL')
    messages = await $.env.get('LC_MESSAGES')
    lang = await $.env.get('LANG')
    term = await $.env.get('TERM')
  } catch {
    // Defaults: English, Unicode glyphs.
  }
  const locale = [all, messages, lang].find(v => v !== undefined && v !== '')
  return { lang: resolveLang([all, messages, lang]), ascii: prefersAsciiGlyphs(term, locale) }
}

/** The raw session id the binary hashes; empty when the host does not say. */
export async function readSessionId($: Pick<EngineInterface, 'env' | 'session'>): Promise<string> {
  try {
    const id = String((await $.session.id()) ?? '')
    if (id !== '') return id
  } catch {
    // Fall through to the environment.
  }
  try {
    return (await $.env.get('CLAUDE_CODE_SESSION_ID')) ?? ''
  } catch {
    return ''
  }
}
