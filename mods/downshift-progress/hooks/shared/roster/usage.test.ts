// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

import { expect, test } from 'claude-code/testing'

import { formatTokens, usageTotal } from './usage'
import { toolUseKey } from './key'

test('usageTotal sums the four fields; missing usage is 0', () => {
  expect(usageTotal({ input_tokens: 1, output_tokens: 2, cache_read_input_tokens: 3, cache_creation_input_tokens: 4 })).toBe(10)
  expect(usageTotal(null)).toBe(0)
})

test('formatTokens: plain, k, M, junk', () => {
  expect([0, 842, 1000, 12_345, 123_456, 1_234_567, Number.NaN, -3].map(formatTokens)).toEqual(['0', '842', '1k', '12.3k', '123k', '1.2M', '0', '0'])
})

test('toolUseKey is the golden sha256 Downshift gives tool call id t1', async () => {
  expect(await toolUseKey('t1')).toBe('c0c017fb725c209fb6150a1a4c15c346e736441dd739277bc61a35b253b22e66')
})
