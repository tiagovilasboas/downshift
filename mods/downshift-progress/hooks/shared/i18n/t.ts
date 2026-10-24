// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

import { EN } from './en'
import { PT_BR } from './pt-BR'
import type { Lang } from './lang'
import type { MessageKey } from './strings'

const TABLES: Readonly<Record<Lang, Readonly<Record<MessageKey, string>>>> = { en: EN, 'pt-BR': PT_BR }

/** The label for `key` in `lang`. Pure: no I/O, no defaults beyond the English table. */
export function t(key: MessageKey, lang: Lang = 'en'): string {
  return TABLES[lang][key]
}
