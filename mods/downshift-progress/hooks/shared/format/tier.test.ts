// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

import { expect, test } from 'claude-code/testing'

import { tierWord } from './tier'

test('catalog tiers map to the band words', () => {
  expect(tierWord('small')).toBe('small')
  expect(tierWord('mid')).toBe('medium')
  expect(tierWord('frontier')).toBe('frontier')
})

test('an unknown tier, even a prototype key, reads as empty', () => {
  expect(tierWord('nope')).toBe('')
  expect(tierWord('constructor')).toBe('')
})
