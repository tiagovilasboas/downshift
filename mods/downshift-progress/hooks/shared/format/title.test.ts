// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

import { expect, test } from 'claude-code/testing'

import { truncateTitle } from './title'

test('a title that fits is returned unchanged', () => {
  expect(truncateTitle('short', 10)).toBe('short')
  expect(truncateTitle('exact', 5)).toBe('exact')
})

test('a long title is cut with an ellipsis inside the width', () => {
  const out = truncateTitle('refactor the billing webhook handler', 12)
  expect(out).toBe('refactor th…')
  expect(Array.from(out).length).toBe(12)
})

test('truncation counts code points, not UTF-16 units', () => {
  expect(truncateTitle('ação-de-cobrança', 6)).toBe('ação-…')
})

test('a width that cannot fit the ellipsis keeps its first characters', () => {
  expect(truncateTitle('abcdef', 1)).toBe('…')
  expect(truncateTitle('abcdef', 0)).toBe('')
  expect(truncateTitle('abcdef', 3, '...')).toBe('...')
  expect(truncateTitle('abcdef', 4, '...')).toBe('a...')
})
