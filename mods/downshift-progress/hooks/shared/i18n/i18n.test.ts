// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

import { expect, test } from 'claude-code/testing'

import { EN } from './en'
import { PT_BR } from './pt-BR'
import { resolveLang } from './lang'
import { MESSAGE_KEYS } from './strings'
import { t } from './t'

test('every key has a non-empty label in both tables', () => {
  for (const key of MESSAGE_KEYS) {
    expect(EN[key].length).toBeGreaterThan(0)
    expect(PT_BR[key].length).toBeGreaterThan(0)
  }
})

test('t returns the English label by default and the pt-BR label on request', () => {
  expect(t('running')).toBe('Running')
  expect(t('running', 'en')).toBe('Running')
  expect(t('running', 'pt-BR')).toBe('Em execução')
  expect(t('downshiftNotFound', 'pt-BR')).toBe('downshift não encontrado')
})

test('resolveLang takes the first candidate it understands', () => {
  expect(resolveLang(['pt-BR', 'en'])).toBe('pt-BR')
  expect(resolveLang([undefined, 'ru', 'pt_BR.UTF-8'])).toBe('pt-BR')
  expect(resolveLang(['ru', 'en-US'])).toBe('en')
  expect(resolveLang(['C.UTF-8'])).toBe('en')
  expect(resolveLang([])).toBe('en')
})
